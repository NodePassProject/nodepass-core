package master

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (m *Master) SaveState() error {
	return m.SaveStateToPath(m.StatePath)
}

func (m *Master) SaveStateToPath(filePath string) error {
	m.StateMu.Lock()
	defer m.StateMu.Unlock()

	persistentData := make(map[string]*Instance)

	m.Instances.Range(func(key, value any) bool {
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
		return fmt.Errorf("SaveStateToPath: mkdirAll failed: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(filePath), "np-*.tmp")
	if err != nil {
		return fmt.Errorf("SaveStateToPath: createTemp failed: %w", err)
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
		return fmt.Errorf("SaveStateToPath: encode failed: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		removeTemp()
		return fmt.Errorf("SaveStateToPath: close temp file failed: %w", err)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		removeTemp()
		return fmt.Errorf("SaveStateToPath: rename temp file failed: %w", err)
	}

	return nil
}

func (m *Master) LoadState() {
	if tmpFiles, _ := filepath.Glob(filepath.Join(filepath.Dir(m.StatePath), "np-*.tmp")); tmpFiles != nil {
		for _, f := range tmpFiles {
			os.Remove(f)
		}
	}

	if _, err := os.Stat(m.StatePath); os.IsNotExist(err) {
		return
	}

	file, err := os.Open(m.StatePath)
	if err != nil {
		m.Logger.Error("LoadState: open file failed: %v", err)
		return
	}
	defer file.Close()

	var persistentData map[string]*Instance
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&persistentData); err != nil {
		m.Logger.Error("LoadState: decode file failed: %v", err)
		return
	}

	for id, instance := range persistentData {
		instance.stopped = make(chan struct{})

		if instance.ID != APIKeyID {
			instance.Status = "stopped"
		}

		if instance.Config == "" && instance.ID != APIKeyID {
			instance.Config = m.GenerateConfigURL(instance)
		}

		if instance.Meta.Tags == nil {
			instance.Meta.Tags = make(map[string]string)
		}

		m.Instances.Store(id, instance)

		if instance.Restart {
			m.Logger.Info("Auto-starting instance: %v [%v]", instance.URL, instance.ID)
			m.StartInstance(instance)
			time.Sleep(BaseDuration)
		}
	}

	m.Logger.Info("Loaded %v instances from %v", len(persistentData), m.StatePath)
}
