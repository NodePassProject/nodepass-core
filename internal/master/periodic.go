package master

import (
	"fmt"
	"time"

	"github.com/NodePassProject/nodepass/internal/common"
)

func (m *Master) StartPeriodicTasks() {
	ticker := time.NewTicker(common.ReloadInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.PerformPeriodicBackup()
			m.PerformPeriodicCleanup()
			m.PerformPeriodicRestart()
		case <-m.PeriodicDone:
			ticker.Stop()
			return
		}
	}
}

func (m *Master) PerformPeriodicBackup() {
	backupPath := fmt.Sprintf("%s.backup", m.StatePath)

	if err := m.SaveStateToPath(backupPath); err != nil {
		m.Logger.Error("PerformPeriodicBackup: backup state failed: %v", err)
	} else {
		m.Logger.Info("State backup saved: %v", backupPath)
	}
}

func (m *Master) PerformPeriodicCleanup() {
	idInstances := make(map[string][]*Instance)
	m.Instances.Range(func(key, value any) bool {
		if id := key.(string); id != APIKeyID {
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
			inst.deleted = true
			if inst.Status != "stopped" {
				m.StopInstance(inst)
			}
			m.Instances.Delete(inst.ID)
		}
	}
}

func (m *Master) PerformPeriodicRestart() {
	var errorInstances []*Instance
	m.Instances.Range(func(key, value any) bool {
		if id := key.(string); id != APIKeyID {
			instance := value.(*Instance)
			if instance.Status == "error" && !instance.deleted {
				errorInstances = append(errorInstances, instance)
			}
		}
		return true
	})

	for _, instance := range errorInstances {
		m.StopInstance(instance)
		time.Sleep(BaseDuration)
		m.StartInstance(instance)
	}
}
