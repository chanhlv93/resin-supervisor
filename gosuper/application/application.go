package application

import (
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/resin-io/resin-supervisor/gosuper/application/updatestatus"
	"github.com/resin-io/resin-supervisor/gosuper/config"
	"github.com/resin-io/resin-supervisor/gosuper/device"
	"github.com/resin-io/resin-supervisor/gosuper/resin"
	"github.com/resin-io/resin-supervisor/gosuper/supermodels"
)

type App supermodels.App

type Manager struct {
	Device               *device.Device
	Apps                 *supermodels.AppsCollection
	Config               *supermodels.Config
	PollInterval         int64
	ResinClient          *resin.Client
	superConfig          config.SupervisorConfig
	updateStatus         *updatestatus.UpdateStatus
	updateTriggerChannel chan bool
}

func NewManager(appsCollection *supermodels.AppsCollection, dbConfig *supermodels.Config, dev *device.Device, superConfig config.SupervisorConfig) (*Manager, error) {
	manager := Manager{Apps: appsCollection, Config: dbConfig, Device: dev, PollInterval: 30000, ResinClient: dev.ResinClient, updateStatus: &dev.UpdateStatus, superConfig: superConfig}
	go manager.UpdateInterval()
	return &manager, nil
}

func (manager *Manager) UpdateInterval() {
	go manager.runUpdates()
	for {
		if manager.Device.Bootstrapped {
			manager.triggerUpdateIfNotRunning()
		}
		time.Sleep(time.Duration(manager.PollInterval) * time.Millisecond)
	}
}

func (manager *Manager) triggerUpdateIfNotRunning() {
	select {
	case manager.updateTriggerChannel <- false:
	default:
	}
}

func (manager *Manager) TriggerUpdate(force bool) {
	go func() {
		manager.updateTriggerChannel <- force
	}()
}

func (manager *Manager) runUpdates() {
	var force bool
	for {
		force = <-manager.updateTriggerChannel
		manager.update(force)
	}
}

// TODO: Implement comparison between remote and local apps
// Consider injection of local env vars, plus special env vars that don't affect updates
func (manager *Manager) compare(remoteApps, localApps []supermodels.App) (obj map[string]interface{}) {
	return obj
}

// TODO: Implement update function
func (manager *Manager) update(force bool) {
	doTheUpdate := func() (err error) {
		var localApps []supermodels.App
		// Get apps from API
		if deviceId, err := manager.Device.GetId(); err != nil {
			return err
		} else if remoteApps, err := manager.ResinClient.GetApps(manager.Device.Uuid, manager.superConfig.RegistryEndpoint, strconv.Itoa(deviceId)); err != nil {
			return err
		} else if err = manager.Apps.List(&localApps); err != nil {
			return err
		} else {
			manager.compare(remoteApps, localApps)
			// Format and compare
			// Apply special actions, boot config
			// Install,remove, update apps (using update strategies)
			return err
		}
	}

	if err := doTheUpdate(); err != nil {
		log.Printf("Error when updating: %s", err)
		manager.updateStatus.FailCount += 1
		manager.updateStatus.UpdateFailed = true
		select {
		case f := <-manager.updateTriggerChannel:
			log.Println("Updating failed, but there is another update scheduled immediately")
			manager.updateTriggerChannel <- f
		default:
			delay := math.Min(math.Pow(2, float64(manager.updateStatus.FailCount)), 30000)
			log.Println("Scheduling another update attempt due to failure: %f", delay)
			manager.scheduleUpdate(delay, force)
		}
	}
	m := map[string]interface{}{"state": "Idle"}
	manager.Device.UpdateState(m)
}

func (manager *Manager) scheduleUpdate(t float64, force bool) {
	go func() {
		<-time.After(time.Duration(t) * time.Millisecond)
		manager.updateTriggerChannel <- force
	}()
}

// TODO: use dockerclient to kill an app
func (app *App) Kill() (err error) {
	log.Printf("Killing app %d", app.AppId)
	return
}

// TODO: use dockerclient to start an app
// TODO: implement logging
// TODO: implement web terminal
func (app *App) Start() (err error) {
	log.Printf("Starting app %d", app.AppId)
	return
}

// TODO: use dockerclient or deltas to fetch an app image
func (app *App) Fetch() (err error) {
	return
}

type AppCallback supermodels.AppCallback

func (app App) DataPath() string {
	return fmt.Sprintf("/mnt/root/resin-data/%d", app.AppId)
}

func (app App) LockPath() string {
	return app.DataPath() + "/resin-updates.lock"
}

func (manager Manager) LockAndDo(app *App, callback AppCallback) error {
	return manager.Apps.GetAndDo((*supermodels.App)(app), func(appFromDB *supermodels.App) error {
		theApp := (*App)(appFromDB)
		path := theApp.LockPath()
		if lock, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0777); err != nil {
			return err
		} else {
			err = callback(appFromDB)
			if e := lock.Close(); e != nil {
				log.Printf("Error closing lockfile: %s\n", e)
			}
			if e := os.Remove(path); e != nil {
				log.Printf("Error releasing lockfile: %s\n", e)
			}
			return err
		}
	})
}
