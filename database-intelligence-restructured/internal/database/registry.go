package database

import (
	"fmt"
	"sync"
)

var (
	driversMu sync.RWMutex
	drivers   = make(map[string]Driver)
)

// RegisterDriver registers a database driver
func RegisterDriver(name string, driver Driver) {
	driversMu.Lock()
	defer driversMu.Unlock()
	
	if driver == nil {
		panic("database: Register driver is nil")
	}
	
	if _, dup := drivers[name]; dup {
		panic("database: Register called twice for driver " + name)
	}
	
	drivers[name] = driver
}

// GetDriver returns a registered driver by name
func GetDriver(name string) (Driver, error) {
	driversMu.RLock()
	defer driversMu.RUnlock()
	
	driver, ok := drivers[name]
	if !ok {
		return nil, fmt.Errorf("database: unknown driver %q (forgotten import?)", name)
	}
	
	return driver, nil
}

// ListDrivers returns a list of registered driver names
func ListDrivers() []string {
	driversMu.RLock()
	defer driversMu.RUnlock()
	
	names := make([]string, 0, len(drivers))
	for name := range drivers {
		names = append(names, name)
	}
	
	return names
}

// DriverFeatures returns the features supported by a driver
func DriverFeatures(name string) ([]Feature, error) {
	driver, err := GetDriver(name)
	if err != nil {
		return nil, err
	}
	
	return driver.SupportedFeatures(), nil
}

// ParseDSN parses a DSN for the specified driver
func ParseDSN(driverName, dsn string) (*Config, error) {
	driver, err := GetDriver(driverName)
	if err != nil {
		return nil, err
	}
	
	return driver.ParseDSN(dsn)
}