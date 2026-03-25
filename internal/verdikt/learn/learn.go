// Package learn adapts Verdikt's quality function over time by correlating
// evaluations with real user reactions: accepted, rephrased, escalated, abandoned.
package learn

import (
	"log"
	"sync"

	"github.com/stockyard-dev/stockyard/internal/verdikt/judge"
	"github.com/stockyard-dev/stockyard/internal/verdikt/store"
)

// The five standard dimensions tracked by Verdikt.
var dimensions = []string{"relevance", "accuracy", "helpfulness", "safety", "tone"}

// Feedback records a user's reaction to an AI response.
type Feedback struct {
	RequestID string `json:"request_id"`
	Reaction  string `json:"reaction"` // accepted, rephrased, escalated, abandoned, thumbs_up, thumbs_down
	Comment   string `json:"comment,omitempty"`
}

// Calibrator learns the relationship between Verdikt scores and user satisfaction.
type Calibrator struct {
	mu            sync.RWMutex
	db            *store.DB
	pairs         []calibrationPair
	maxPairs      int
	biasShift     float64            // legacy single bias (kept for backward compat)
	dimensionBias map[string]float64 // per-dimension bias adjustments
}

type calibrationPair struct {
	Score      float64
	Dimensions map[string]float64
	Reaction   string
	Happy      bool // derived from reaction
}

// New creates a calibrator. If db is non-nil, feedback and calibration
// state are persisted to SQLite and restored on startup.
func New(db *store.DB) *Calibrator {
	c := &Calibrator{
		db:            db,
		maxPairs:      2000,
		dimensionBias: make(map[string]float64),
	}
	if db != nil {
		c.loadFromDB()
	}
	return c
}

// loadFromDB restores per-dimension bias from the calibration table.
func (c *Calibrator) loadFromDB() {
	// Load legacy single bias
	v, err := c.db.GetCalibration("bias_shift")
	if err != nil {
		log.Printf("verdikt/learn: failed to load calibration: %v", err)
	} else {
		c.biasShift = v
	}

	// Load per-dimension biases
	for _, dim := range dimensions {
		key := "bias_" + dim
		v, err := c.db.GetCalibration(key)
		if err != nil {
			log.Printf("verdikt/learn: failed to load %s: %v", key, err)
			continue
		}
		c.dimensionBias[dim] = v
	}
}

// RecordFeedback correlates a user reaction with the evaluation score.
func (c *Calibrator) RecordFeedback(eval *judge.Evaluation, fb Feedback) {
	c.mu.Lock()
	defer c.mu.Unlock()

	happy := fb.Reaction == "accepted" || fb.Reaction == "thumbs_up"
	c.pairs = append(c.pairs, calibrationPair{
		Score: eval.Score,
		Dimensions: map[string]float64{
			"relevance":   eval.Dimensions.Relevance,
			"accuracy":    eval.Dimensions.Accuracy,
			"helpfulness": eval.Dimensions.Helpfulness,
			"safety":      eval.Dimensions.Safety,
			"tone":        eval.Dimensions.Tone,
		},
		Reaction: fb.Reaction,
		Happy:    happy,
	})
	if len(c.pairs) > c.maxPairs {
		c.pairs = c.pairs[len(c.pairs)-c.maxPairs:]
	}

	if c.db != nil {
		_ = c.db.SaveFeedback(fb.RequestID, fb.Reaction, fb.Comment, eval.Score)
	}

	// Recalculate bias
	c.recalibrate()
}

// AdjustScore applies learned calibration to a raw score.
func (c *Calibrator) AdjustScore(raw float64) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	adjusted := raw + c.biasShift
	if adjusted < 0 {
		adjusted = 0
	}
	if adjusted > 1 {
		adjusted = 1
	}
	return adjusted
}

// AdjustDimension applies per-dimension calibration bias.
func (c *Calibrator) AdjustDimension(dim string, raw float64) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	adjusted := raw + c.dimensionBias[dim]
	if adjusted < 0 {
		adjusted = 0
	}
	if adjusted > 1 {
		adjusted = 1
	}
	return adjusted
}

// recalibrate computes per-dimension bias from feedback data.
// For each dimension: if users are happy with responses that scored low on
// that dimension, the bias shifts up; if unhappy with high scores, down.
func (c *Calibrator) recalibrate() {
	if len(c.pairs) < 10 {
		return
	}

	// Per-dimension calibration
	for _, dim := range dimensions {
		var happySum, unhappySum float64
		var happyCount, unhappyCount int

		for _, p := range c.pairs {
			dimScore, ok := p.Dimensions[dim]
			if !ok {
				continue
			}
			if p.Happy {
				happySum += dimScore
				happyCount++
			} else {
				unhappySum += dimScore
				unhappyCount++
			}
		}

		if happyCount == 0 || unhappyCount == 0 {
			continue
		}

		happyAvg := happySum / float64(happyCount)
		unhappyAvg := unhappySum / float64(unhappyCount)

		idealMidpoint := 0.65
		actualMidpoint := (happyAvg + unhappyAvg) / 2
		c.dimensionBias[dim] = (idealMidpoint - actualMidpoint) * 0.5

		if c.db != nil {
			_ = c.db.SetCalibration("bias_"+dim, c.dimensionBias[dim])
		}
	}

	// Also maintain the legacy overall bias
	var happyScoreSum, unhappyScoreSum float64
	var happyCount, unhappyCount int
	for _, p := range c.pairs {
		if p.Happy {
			happyScoreSum += p.Score
			happyCount++
		} else {
			unhappyScoreSum += p.Score
			unhappyCount++
		}
	}
	if happyCount > 0 && unhappyCount > 0 {
		happyAvg := happyScoreSum / float64(happyCount)
		unhappyAvg := unhappyScoreSum / float64(unhappyCount)
		idealMidpoint := 0.65
		actualMidpoint := (happyAvg + unhappyAvg) / 2
		c.biasShift = (idealMidpoint - actualMidpoint) * 0.5
		if c.db != nil {
			_ = c.db.SetCalibration("bias_shift", c.biasShift)
		}
	}
}

// CalibrationStats shows how well Verdikt correlates with user satisfaction.
type CalibrationStats struct {
	TotalFeedback  int                `json:"total_feedback"`
	HappyRate      float64            `json:"happy_rate"`
	BiasShift      float64            `json:"bias_shift"`
	DimensionBias  map[string]float64 `json:"dimension_bias"`
	Accuracy       float64            `json:"accuracy"` // % of times high score = happy, low score = unhappy
}

func (c *Calibrator) Stats() CalibrationStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	dimBias := make(map[string]float64)
	for k, v := range c.dimensionBias {
		dimBias[k] = v
	}

	s := CalibrationStats{
		TotalFeedback: len(c.pairs),
		BiasShift:     c.biasShift,
		DimensionBias: dimBias,
	}

	if len(c.pairs) == 0 {
		return s
	}

	var happy, correct int
	for _, p := range c.pairs {
		if p.Happy {
			happy++
		}
		if (p.Score >= 0.65 && p.Happy) || (p.Score < 0.65 && !p.Happy) {
			correct++
		}
	}

	s.HappyRate = float64(happy) / float64(len(c.pairs))
	s.Accuracy = float64(correct) / float64(len(c.pairs))

	return s
}
