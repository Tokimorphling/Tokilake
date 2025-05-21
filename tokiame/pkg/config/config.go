package config

import (
	"bytes"
	"fmt"

	// "log"
	"os"
	"path/filepath"
	"sync"

	// "time" // No longer directly used in this file for the demo sleep

	"github.com/BurntSushi/toml"
	"github.com/fsnotify/fsnotify"

	// You would import your actual generated protobuf package here.
	// For example:
	// pb "your_module_path/gen/go/open/v1"
	//
	// For this example, we'll define a placeholder for ModelDetails
	// and a placeholder 'pb' package to make the code runnable.
	// **IMPORTANT**: Replace these placeholders with your actual imports and struct definitions.
	// If your main package also needs to know about ModelDetails, it would use config.ModelDetails.
	// pb "tokiame/internal/pb/tokilake/v1" // Placeholder import
	"tokiame/pkg/log"
	// pb "tokiame/internal/pb/tokilake/v1"
)

// --- Placeholder for your actual protobuf-generated ModelDetails struct ---
// IMPORTANT: In your actual project, you should REMOVE this placeholder definition
// and ensure your `pb` import points to your actual generated code.
// This struct is now part of the `config` package.
type ModelDetails struct {
	Id                string            `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty" toml:"id"`
	Description       string            `protobuf:"bytes,2,opt,name=description,proto3" json:"description,omitempty" toml:"description,omitempty"`
	Type              string            `protobuf:"bytes,3,opt,name=type,proto3" json:"type,omitempty" toml:"type"`
	Capabilities      map[string]string `protobuf:"bytes,4,rep,name=capabilities,proto3" json:"capabilities,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value" toml:"capabilities,omitempty"`
	BackendEngine     string            `protobuf:"bytes,5,opt,name=backend_engine,json=backendEngine,proto3" json:"backend_engine,omitempty" toml:"backend_engine"`
	BackendBase       string            `json:"backend_base,omitempty" toml:"backend_base"`
	Status            string            `protobuf:"bytes,6,opt,name=status,proto3" json:"status,omitempty" toml:"status"`                                                                          // "LOADING", "READY", "ERROR"
	CurrentLoadFactor int32             `protobuf:"varint,7,opt,name=current_load_factor,json=currentLoadFactor,proto3" json:"current_load_factor,omitempty" toml:"current_load_factor,omitempty"` // e.g., 0-100
}

// --- End of Placeholder ---

// ModelConfig is a wrapper struct to match the top-level structure of your TOML file.
// It uses the ModelDetails type defined within this package.
type ModelConfig struct {
	SupportedModels []*ModelDetails `toml:"supported_models"`
}

// Manager handles the application configuration, including loading, watching for changes, and saving.
type Manager struct {
	filePath string
	current  *ModelConfig
	mu       sync.RWMutex
	watcher  *fsnotify.Watcher
	done     chan struct{} // Channel to signal the watcher to stop
}

// NewManager creates a new configuration Manager.
// It performs an initial load of the configuration from the given filePath.
// It also starts a goroutine to watch the file for changes.
func NewManager(filePath string) (*Manager, error) {
	// tokilakev1.Acknowledgement

	m := &Manager{
		filePath: filePath,
		done:     make(chan struct{}),
	}

	// Initial load
	if err := m.load(); err != nil {
		return nil, fmt.Errorf("initial config load failed: %w", err)
	}
	log.Infof("Configuration initially loaded successfully from '%s'.", m.filePath)
	m.PrintState()

	// Initialize and start watcher
	var err error
	m.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	go m.watchFile() // Start watching in a goroutine

	configFileDir := filepath.Dir(m.filePath)
	if err := m.watcher.Add(configFileDir); err != nil {
		m.watcher.Close() // Clean up watcher if adding path fails
		return nil, fmt.Errorf("failed to add config file directory ('%s') to watcher: %w", configFileDir, err)
	}
	log.Infof("Watching for changes in directory '%s' for file '%s'...", configFileDir, filepath.Base(m.filePath))

	return m, nil
}

// load reads the TOML file and updates the manager's current configuration.
// This is an internal method; external users trigger reloads via file changes.
func (m *Manager) load() error {
	tomlData, err := os.ReadFile(m.filePath)
	if err != nil {
		return fmt.Errorf("error reading TOML config file '%s': %w", m.filePath, err)
	}

	var conf ModelConfig
	if _, err := toml.Decode(string(tomlData), &conf); err != nil {
		return fmt.Errorf("error decoding TOML data from '%s': %w", m.filePath, err)
	}

	m.mu.Lock()
	m.current = &conf
	m.mu.Unlock()
	return nil
}

// Get returns a pointer to the current configuration.
// The returned pointer should be treated as read-only.
// For modifications, use specific methods like ModifyModelStatus.
func (m *Manager) Get() *ModelConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Consider returning a deep copy if external modifications are a concern
	// and not managed through dedicated methods. For now, RLock protects reads.
	return m.current
}

// PrintState logs a summary of the current configuration.
func (m *Manager) PrintState() {
	cfg := m.Get() // Use the Get method to ensure thread-safe access
	if cfg == nil {
		log.Info("Config Manager: Current configuration is nil.")
		return
	}
	log.Infof("Config Manager: Current configuration has %d models.", len(cfg.SupportedModels))
	for _, model := range cfg.SupportedModels {
		log.Infof("  Model ID: %s, Status: %s, Load: %d%%", model.Id, model.Status, model.CurrentLoadFactor)
	}
}

// watchFile continuously monitors the configuration file for changes.
func (m *Manager) watchFile() {
	configFileName := filepath.Base(m.filePath)

	for {
		select {
		case <-m.done: // Check if we need to stop watching
			log.Info("Config Manager: Stopping file watcher.")
			return
		case event, ok := <-m.watcher.Events:
			if !ok {
				log.Info("Config Manager: File watcher events channel closed.")
				return
			}
			if filepath.Base(event.Name) == configFileName {
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					log.Infof("Config Manager: File event: %s for %s", event.Op.String(), event.Name)
					log.Info("Config Manager: Reloading configuration...")
					if err := m.load(); err != nil {
						log.Infof("Config Manager: Error reloading config file '%s': %v. Keeping previous configuration.", m.filePath, err)
					} else {
						log.Infof("Config Manager: Configuration reloaded successfully from '%s'.", m.filePath)
						m.PrintState() // Optional: print state after successful reload
					}
				}
			}
		case err, ok := <-m.watcher.Errors:
			if !ok {
				log.Info("Config Manager: File watcher errors channel closed.")
				return
			}
			log.Infof("Config Manager: File watcher error: %v", err)
		}
	}
}

// Save writes the current configuration back to the TOML file.
// WARNING: This will overwrite the existing file and may lose comments/formatting.
func (m *Manager) Save() error {
	m.mu.RLock() // Lock for reading the current config
	configToSave := m.current
	m.mu.RUnlock()

	if configToSave == nil {
		return fmt.Errorf("config manager: cannot save nil configuration")
	}

	var buffer bytes.Buffer
	encoder := toml.NewEncoder(&buffer)
	if err := encoder.Encode(configToSave); err != nil {
		return fmt.Errorf("config manager: error encoding configuration to TOML: %w", err)
	}

	if err := os.WriteFile(m.filePath, buffer.Bytes(), 0644); err != nil {
		return fmt.Errorf("config manager: error writing configuration to file '%s': %w", m.filePath, err)
	}

	log.Infof("Config Manager: Configuration successfully saved to '%s'.", m.filePath)
	return nil
}

// ModifyModelStatus updates the status and load factor of a specific model
// and then saves the entire configuration.
func (m *Manager) ModifyModelStatus(modelID string, newStatus string, newLoadFactor int32) error {
	m.mu.Lock() // Acquire a full lock for modification and subsequent save
	defer m.mu.Unlock()

	if m.current == nil {
		return fmt.Errorf("config manager: current configuration is not loaded, cannot modify")
	}

	found := false
	for _, model := range m.current.SupportedModels {
		if model.Id == modelID {
			log.Infof("Config Manager: Updating model '%s': Status from '%s' to '%s', Load from %d to %d",
				model.Id, model.Status, newStatus, model.CurrentLoadFactor, newLoadFactor)
			model.Status = newStatus
			model.CurrentLoadFactor = newLoadFactor
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("config manager: model with ID '%s' not found in current configuration", modelID)
	}

	// Now save the modified currentConfig
	// Re-encode the m.current which is already locked
	var buffer bytes.Buffer
	encoder := toml.NewEncoder(&buffer)
	if err := encoder.Encode(m.current); err != nil {
		return fmt.Errorf("config manager: error encoding modified configuration to TOML: %w", err)
	}

	if err := os.WriteFile(m.filePath, buffer.Bytes(), 0644); err != nil {
		return fmt.Errorf("config manager: error writing modified configuration to file '%s': %w", m.filePath, err)
	}

	log.Infof("Config Manager: Model '%s' updated and configuration saved to '%s'.", modelID, m.filePath)
	return nil
}

// Close stops the file watcher and cleans up resources.
func (m *Manager) Close() {
	log.Info("Config Manager: Closing...")
	close(m.done) // Signal the watcher goroutine to stop
	if m.watcher != nil {
		m.watcher.Close()
	}
}
