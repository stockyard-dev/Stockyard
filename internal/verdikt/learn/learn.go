// Package learn adapts Verdikt's quality function over time by correlating
// evaluations with real user reactions: accepted, rephrased, escalated, abandoned.
package learn

import (
	"sync"

	"github.com/stockyard-dev/stockyard/internal/verdikt/judge"
)

// Feedback records a user's reaction to an AI response.
type Feedback struct {
	RequestID string  `json:"request_id"`
	Reaction  string  `json:"reaction"` // accepted, rephrased, escalated, abandoned, thumbs_up, thumbs_down
	Comment   string  `json:"comment,omitempty"`
}

// Calibrator learns the relationship between Verdikt scores and user satisfaction.
type Calibrator struct {
	mu        sync.RWMutex
	pairs     []calibrationPair
	maxPairs  int
	biasShift float64 // learned adjustment to raw scores
}

type calibrationPair struct {
	Score    float64
	Reaction string
	Happy    bool // derived from reaction
}

// New creates a calibrator.
func New() *Calibrator {
	return &Calibrator{maxPairs: 2000}
}

// RecordFeedback correlates a user reaction with the evaluation score.
func (c *Calibrator) RecordFeedback(eval *judge.Evaluation, fb Feedback) {
	c.mu.Lock()
	defer c.mu.Unlock()

	happy := fb.Reaction == "accepted" || fb.Reaction == "thumbs_up"
	c.pairs = append(c.pairs, calibrationPair{
		Score: eval.Score, Reaction: fb.Reaction, Happy: happy,
	})
	if len(c.pairs) > c.maxPairs {
		c.pairs = c.pairs[len(c.pairs)-c.maxPairs:]
	}

	// Recalculate bias
	c.recalibrate()
}

// AdjustScore applies learned calibration to a raw score.
func (c *Calibrator) AdjustScore(raw float64) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	adjusted := raw + c.biasShift
	if adjusted < 0 { adjusted = 0 }
	if adjusted > 1 { adjusted = 1 }
	return adjusted
}

// recalibrate computes bias from feedback data.
// If users are happy with responses Verdikt scored low, bias shifts up.
// If users are unhappy with responses Verdikt scored high, bias shifts down.
func (c *Calibrator) recalibrate() {
	if len(c.pairs) < 10 {
		return
	}

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

	if happyCount == 0 || unhappyCount == 0 {
		return
	}

	happyAvg := happyScoreSum / float64(happyCount)
	unhappyAvg := unhappyScoreSum / float64(unhappyCount)

	// If happy users have lower scores than expected, Verdikt is too harsh
	// If unhappy users have higher scores, Verdikt is too lenient
	idealMidpoint := 0.65
	actualMidpoint := (happyAvg + unhappyAvg) / 2

	c.biasShift = (idealMidpoint - actualMidpoint) * 0.5 // conservative adjustment
}

// CalibrationStats shows how well Verdikt correlates with user satisfaction.
type CalibrationStats struct {
	TotalFeedback int     `json:"total_feedback"`
	HappyRate     float64 `json:"happy_rate"`
	BiasShift     float64 `json:"bias_shift"`
	Accuracy      float64 `json:"accuracy"` // % of times high score = happy, low score = unhappy
}

func (c *Calibrator) Stats() CalibrationStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	s := CalibrationStats{
		TotalFeedback: len(c.pairs),
		BiasShift:     c.biasShift,
	}

	if len(c.pairs) == 0 {
		return s
	}

	var happy, correct int
	for _, p := range c.pairs {
		if p.Happy {
			happy++
		}
		// "Correct" = high score & happy, OR low score & unhappy
		if (p.Score >= 0.65 && p.Happy) || (p.Score < 0.65 && !p.Happy) {
			correct++
		}
	}

	s.HappyRate = float64(happy) / float64(len(c.pairs))
	s.Accuracy = float64(correct) / float64(len(c.pairs))

	return s
}
