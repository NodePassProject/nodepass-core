package master

import (
	"fmt"
	"time"

	"github.com/NodePassProject/nodepass/internal/common"
)

func (m *Master) startPeriodicTasks() {
	ticker := time.NewTicker(common.ReloadInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.performPeriodicBackup()
			m.performPeriodicCleanup()
			m.performPeriodicRestart()
		case <-m.periodicDone:
			ticker.Stop()
			return
		}
	}
}

func (m *Master) performPeriodicBackup() {
	backupPath := fmt.Sprintf("%s.backup", m.statePath)

	if err := m.saveStateToPath(backupPath); err != nil {
		m.Logger.Error("performPeriodicBackup: backup state failed: %v", err)
	} else {
		m.Logger.Info("State backup saved: %v", backupPath)
	}
}

func (m *Master) performPeriodicCleanup() {
	idInstances := make(map[string][]*Instance)
	m.instances.Range(func(key, value any) bool {
		if id := key.(string); id != apiKeyID {
			idInstances[id] = append(idInstances[id], value.(*Instance))
		}
		return true
	})

	for _, instances := range idInstances {
		if len(instances) <= 1 {
			continue
		}

		keepIdx := 0
		for i, inst := range instances {
			if inst.Status == "running" && instances[keepIdx].Status != "running" {
				keepIdx = i
			}
		}

		for i, inst := range instances {
			if i == keepIdx {
				continue
			}
			inst.Deleted = true
			if inst.Status != "stopped" {
				m.stopInstance(inst)
			}
			m.instances.Delete(inst.ID)
		}
	}
}

func (m *Master) performPeriodicRestart() {
	var errorInstances []*Instance
	m.instances.Range(func(key, value any) bool {
		if id := key.(string); id != apiKeyID {
			instance := value.(*Instance)
			if instance.Status == "error" && !instance.Deleted {
				errorInstances = append(errorInstances, instance)
			}
		}
		return true
	})

	for _, instance := range errorInstances {
		m.stopInstance(instance)
		time.Sleep(baseDuration)
		m.startInstance(instance)
	}
}
