package apiserver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DB is the API backend storage using in-memory maps with JSON persistence.
// Simple, fast, zero-dependency. Suitable for the scale we're at (<10K customers).
type DB struct {
	mu         sync.RWMutex
	customers  map[string]*Customer
	licenses   []*LicenseRecord
	webhooks   map[string]bool
	nextCustID int64
	nextLicID  int64
	path       string
}

// Customer represents a Stripe customer record.
type Customer struct {
	ID               int64     `json:"id"`
	StripeCustomerID string    `json:"stripe_customer_id"`
	Email            string    `json:"email"`
	Name             string    `json:"name,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// LicenseRecord represents a stored license key.
type LicenseRecord struct {
	ID                   int64     `json:"id"`
	CustomerID           int64     `json:"customer_id"`
	StripeCustomerID     string    `json:"stripe_customer_id"`
	StripeSubscriptionID string    `json:"stripe_subscription_id,omitempty"`
	Product              string    `json:"product"`
	Tier                 string    `json:"tier"`
	LicenseKey           string    `json:"license_key"`
	Status               string    `json:"status"`
	Email                string    `json:"email"`
	CreatedAt            time.Time `json:"created_at"`
	ExpiresAt            time.Time `json:"expires_at,omitempty"`
}

type dbSnapshot struct {
	Customers  map[string]*Customer `json:"customers"`
	Licenses   []*LicenseRecord     `json:"licenses"`
	NextCustID int64                `json:"next_cust_id"`
	NextLicID  int64                `json:"next_lic_id"`
}

// OpenDB opens or creates the API backend database.
func OpenDB(path string) (*DB, error) {
	db := &DB{
		customers:  make(map[string]*Customer),
		webhooks:   make(map[string]bool),
		nextCustID: 1,
		nextLicID:  1,
	}
	if path != "" && path != ":memory:" {
		db.path = path
		if data, err := os.ReadFile(path); err == nil {
			var snap dbSnapshot
			if err := json.Unmarshal(data, &snap); err == nil {
				db.customers = snap.Customers
				db.licenses = snap.Licenses
				db.nextCustID = snap.NextCustID
				db.nextLicID = snap.NextLicID
			}
		}
		if db.customers == nil {
			db.customers = make(map[string]*Customer)
		}
	}
	return db, nil
}

func (d *DB) Close() error { return d.persist() }

func (d *DB) persist() error {
	if d.path == "" {
		return nil
	}
	d.mu.RLock()
	snap := dbSnapshot{Customers: d.customers, Licenses: d.licenses, NextCustID: d.nextCustID, NextLicID: d.nextLicID}
	d.mu.RUnlock()
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(d.path); dir != "" && dir != "." {
		os.MkdirAll(dir, 0755)
	}
	return os.WriteFile(d.path, data, 0644)
}

func (d *DB) UpsertCustomer(stripeID, email, name string) (*Customer, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if existing, ok := d.customers[stripeID]; ok {
		existing.Email = email
		if name != "" {
			existing.Name = name
		}
		return existing, nil
	}
	c := &Customer{ID: d.nextCustID, StripeCustomerID: stripeID, Email: email, Name: name, CreatedAt: time.Now()}
	d.nextCustID++
	d.customers[stripeID] = c
	return c, nil
}

func (d *DB) GetCustomerByStripeID(stripeID string) (*Customer, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	c, ok := d.customers[stripeID]
	if !ok {
		return nil, fmt.Errorf("customer not found: %s", stripeID)
	}
	return c, nil
}

func (d *DB) GetCustomerByEmail(email string) (*Customer, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, c := range d.customers {
		if c.Email == email {
			return c, nil
		}
	}
	return nil, fmt.Errorf("customer not found: %s", email)
}

func (d *DB) CreateLicense(rec *LicenseRecord) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	rec.ID = d.nextLicID
	rec.CreatedAt = time.Now()
	d.nextLicID++
	d.licenses = append(d.licenses, rec)
	return nil
}

func (d *DB) GetLicenseByKey(key string) (*LicenseRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, l := range d.licenses {
		if l.LicenseKey == key {
			return l, nil
		}
	}
	return nil, fmt.Errorf("license not found")
}

func (d *DB) GetLicensesBySubscription(subID string) ([]*LicenseRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var result []*LicenseRecord
	for _, l := range d.licenses {
		if l.StripeSubscriptionID == subID {
			result = append(result, l)
		}
	}
	return result, nil
}

func (d *DB) GetLicensesByCustomer(stripeCustomerID string) ([]*LicenseRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var result []*LicenseRecord
	for _, l := range d.licenses {
		if l.StripeCustomerID == stripeCustomerID {
			result = append(result, l)
		}
	}
	return result, nil
}

func (d *DB) UpdateLicenseStatus(subID, status string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, l := range d.licenses {
		if l.StripeSubscriptionID == subID {
			l.Status = status
		}
	}
	return nil
}

func (d *DB) UpdateLicenseTier(subID, tier string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, l := range d.licenses {
		if l.StripeSubscriptionID == subID {
			l.Tier = tier
		}
	}
	return nil
}

func (d *DB) UpdateLicenseStatusByID(id int64, status string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, l := range d.licenses {
		if l.ID == id {
			l.Status = status
			break
		}
	}
	return nil
}

func (d *DB) IsWebhookProcessed(eventID string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.webhooks[eventID]
}

func (d *DB) MarkWebhookProcessed(eventID, eventType string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.webhooks[eventID] = true
	return nil
}

func (d *DB) Stats() map[string]any {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var active, canceled int64
	tierCounts := map[string]int64{}
	for _, l := range d.licenses {
		switch l.Status {
		case "active":
			active++
			tierCounts[l.Tier]++
		case "canceled":
			canceled++
		}
	}
	return map[string]any{
		"customers":         int64(len(d.customers)),
		"active_licenses":   active,
		"total_licenses":    int64(len(d.licenses)),
		"canceled_licenses": canceled,
		"by_tier":           tierCounts,
	}
}
