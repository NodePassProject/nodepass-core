package master

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (m *Master) saveState() error {
	return m.saveStateToPath(m.statePath)
}

func (m *Master) saveStateToPath(filePath string) error {
	if !m.stateMu.TryLock() {
		return nil
	}
	defer m.stateMu.Unlock()

	persistentData := make(map[string]*Instance)

	m.instances.Range(func(key, value any) bool {
		instance := value.(*Instance)
		persistentData[key.(string)] = instance
		return true
	})

	if len(persistentData) == 0 {
		if _, err := os.Stat(filePath); err == nil {
			return os.Remove(filePath)
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("saveStateToPath: mkdirAll failed: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(filePath), "np-*.tmp")
	if err != nil {
		return fmt.Errorf("saveStateToPath: createTemp failed: %w", err)
	}
	tempPath := tempFile.Name()

	removeTemp := func() {
		if _, err := os.Stat(tempPath); err == nil {
			os.Remove(tempPath)
		}
	}

	encoder := gob.NewEncoder(tempFile)
	if err := encoder.Encode(persistentData); err != nil {
		tempFile.Close()
		removeTemp()
		return fmt.Errorf("saveStateToPath: encode failed: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		removeTemp()
		return fmt.Errorf("saveStateToPath: close temp file failed: %w", err)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		removeTemp()
		return fmt.Errorf("saveStateToPath: rename temp file failed: %w", err)
	}

	return nil
}

func (m *Master) loadState() {
	if tmpFiles, _ := filepath.Glob(filepath.Join(filepath.Dir(m.statePath), "np-*.tmp")); tmpFiles != nil {
		for _, f := range tmpFiles {
			os.Remove(f)
		}
	}

	if _, err := os.Stat(m.statePath); os.IsNotExist(err) {
		return
	}

	file, err := os.Open(m.statePath)
	if err != nil {
		m.Logger.Error("loadState: open file failed: %v", err)
		return
	}
	defer file.Close()

	var persistentData map[string]*Instance
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&persistentData); err != nil {
		m.Logger.Error("loadState: decode file failed: %v", err)
		return
	}

	for id, instance := range persistentData {
		instance.Stopped = make(chan struct{})

		if instance.ID != apiKeyID {
			instance.Status = "stopped"
		}

		if instance.Config == "" && instance.ID != apiKeyID {
			instance.Config = m.generateConfigURL(instance)
		}

		if instance.Meta.Tags == nil {
			instance.Meta.Tags = make(map[string]string)
		}

		m.instances.Store(id, instance)

		if instance.Restart {
			m.Logger.Info("Auto-starting instance: %v [%v]", instance.URL, instance.ID)
			m.startInstance(instance)
			time.Sleep(baseDuration)
		}
	}

	m.Logger.Info("Loaded %v instances from %v", len(persistentData), m.statePath)
}
