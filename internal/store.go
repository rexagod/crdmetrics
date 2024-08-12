package internal

import (
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// StoreType implements the k8s.io/client-go/tools/cache.StoreType interface. The cache.Reflector uses the cache.StoreType to
// operate on the store.metrics map with the various metric families and their metrics based on the associated object's
// events.
type StoreType struct {

	// logger is the store's logger.
	logger klog.Logger

	// mutex is a binary semaphore that is used to prevent RW races w.r.t. the store's internal metric map.
	mutex sync.RWMutex

	// metrics is the store's internal metric map. It is indexed by the object's UID and contains a slice of
	// metric families, which in turn contain a slice of metrics.
	metrics map[types.UID][]string

	// headers contain the type and help text for each metric family, corresponding to the store's internal
	// metric map's keys.
	headers []string

	// Exported attributes that each store is associated with, used for unmarshalling the configuration.
	Group        string `yaml:"g"`
	Version      string `yaml:"v"`
	Kind         string `yaml:"k"`
	ResourceName string `yaml:"r"`
	Selectors    struct {
		Label string `yaml:"label,omitempty"`
		Field string `yaml:"field,omitempty"`
	} `yaml:"selectors,omitempty"`
	Families []*FamilyType `yaml:"families"`
}

// newStore returns a new store.
func newStore(logger klog.Logger, headers []string, families []*FamilyType) *StoreType {
	return &StoreType{
		logger:   logger,
		metrics:  map[types.UID][]string{},
		headers:  headers,
		Families: families,
	}
}

// Add adds the given object to the accumulator associated with its key.
func (s *StoreType) Add(objectI interface{}) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Cast into a typed object.
	object, err := meta.Accessor(objectI)
	if err != nil {
		return fmt.Errorf("error casting object interface: %w", err)
	}

	// Generate metrics from the object.
	familyMetrics := make([]string, len(s.Families))
	for i, f := range s.Families {
		familyMetrics[i] = f.raw()
		s.logger.V(4).Info("Add", "family", f.Name, "metrics", familyMetrics[i])
	}

	// Store the generated metrics.
	s.logger.V(2).Info("Add", "key", klog.KObj(object))
	s.metrics[object.GetUID()] = familyMetrics

	return nil
}

// Update updates the given object in the accumulator associated with its key.
func (s *StoreType) Update(objectI interface{}) error {
	s.logger.V(2).Info("Update", "defer", "Add")
	return s.Add(objectI)
}

// Delete deletes the given object from the accumulator associated with its key.
func (s *StoreType) Delete(objectI interface{}) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Cast into a typed object.
	object, err := meta.Accessor(objectI)
	if err != nil {
		return fmt.Errorf("error casting object interface: %w", err)
	}

	// Delete the object's metrics.
	s.logger.V(2).Info("Delete", "key", klog.KObj(object))
	s.logger.V(4).Info("Delete", "metrics", s.metrics[object.GetUID()])
	delete(s.metrics, object.GetUID())

	return nil
}

// List returns a list of all the currently non-empty accumulators.
func (s *StoreType) List() []interface{} {
	return nil
}

// ListKeys returns a list of all the keys of the currently non-empty accumulators.
func (s *StoreType) ListKeys() []string {
	return nil
}

// Get returns the accumulator associated with the given object's key.
func (s *StoreType) Get(_ interface{}) (interface{}, bool, error) {
	return nil, false, nil
}

// GetByKey returns the accumulator associated with the given key.
func (s *StoreType) GetByKey(_ string) (interface{}, bool, error) {
	return nil, false, nil
}

// Replace will delete the contents of the store, using instead the given list. store takes ownership of the list, you
// should not reference it after calling this function.
func (s *StoreType) Replace(objectIs []interface{}, _ string) error {
	s.logger.V(2).Info("Replace", "defer", "Add")
	s.metrics = map[types.UID][]string{}
	for _, o := range objectIs {
		err := s.Add(o)
		if err != nil {
			return err
		}
	}

	return nil
}

// Resync is meaningless in the terms appearing here but has meaning in some implementations that have non-trivial
// additional behavior (e.g., DeltaFIFO).
func (s *StoreType) Resync() error {
	return nil
}
