package system

import "os"

// Environment is the interface for the environment.
type Environment interface {
	Get(key string) (string, bool)
	UserHomeDir() (string, error)
}

// env is the default implementation of the Environment interface.
type env struct{}

// NewEnvironment creates a new Environment.
func NewEnvironment() Environment {
	return &env{}
}

// Get gets the value of the environment variable with the given key.
func (e *env) Get(key string) (string, bool) {
	return os.LookupEnv(key)
}

// UserHomeDir returns the home directory of the current user.
func (e *env) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}
