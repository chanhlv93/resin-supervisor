package application

import (
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/resin-io/resin-supervisor/gosuper/application/updatestatus"
	"github.com/resin-io/resin-supervisor/gosuper/config"
	"github.com/resin-io/resin-supervisor/gosuper/device"
	//"github.com/resin-io/resin-supervisor/gosuper/resin"
	//"github.com/resin-io/resin-supervisor/gosuper/cliclient"
	"github.com/resin-io/resin-supervisor/gosuper/supermodels"
	"github.com/resin-io/resin-supervisor/gosuper/Godeps/_workspace/src/github.com/samalba/dockerclient"
	"github.com/resin-io/resin-supervisor/gosuper/cliclient"
	"strconv"
)

const (
	DeviceUpdate = "updating"
	DeviceUpdateFinish = "updated"
)

type App supermodels.App

type Manager struct {
	Device               *device.Device
	Apps                 *supermodels.AppsCollection
	Config               *supermodels.Config
	PollInterval         int64
	CliClient            cliclient.Client
	superConfig          config.SupervisorConfig
	updateStatus         *updatestatus.UpdateStatus
	updateTriggerChannel chan bool
}

func NewManager(appsCollection *supermodels.AppsCollection, dbConfig *supermodels.Config, dev *device.Device, superConfig config.SupervisorConfig) (*Manager, error) {
	manager := Manager{Apps: appsCollection, Config: dbConfig, Device: dev, PollInterval: 30000, CliClient: dev.CliClient, updateStatus: &dev.UpdateStatus, superConfig: superConfig}
	//go manager.initApp(manager.superConfig.DockerSocket)
	if !manager.Device.Bootstrapped {
		go manager.loadPreloadedApps()
	}

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
		if remoteApps, err := manager.CliClient.GetApps(strconv.Itoa(manager.Device.Config.ApplicationId)); err != nil {
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

// TODO: Create function to pull & run sample app
func (manager *Manager)loadPreloadedApps() {
	//log.Println("-------> start application default, call api update")
	manager.CliClient.UpdateState(manager.Device.Config.ApplicationId, manager.Device.Config.DeviceId, DeviceUpdate)
	var err error
	appDefault := supermodels.App{AppId: manager.Device.Config.ApplicationId, ContainerId:"", Commit:"", Env:nil, ImageId:"chanhlv93/cli-app"}
	if err = manager.Apps.CreateOrUpdate(&appDefault); err != nil {
		log.Println(err)
		return
	}

	var appName = "cli-app"
	var imageName string = "chanhlv93/" + appName
	//Hard code docker hub user
	authConfig := dockerclient.AuthConfig{Username:"chanhlv93", Password:"chanhlove1993"}
	if docker, err := dockerclient.NewDockerClient("unix://" + manager.superConfig.DockerSocket, nil); err != nil {
		log.Println(err)
	} else if err = docker.PullImage(imageName, &authConfig); err != nil {
		log.Println(err)
	} else {
		containerConfig := &dockerclient.ContainerConfig{
			Image: imageName,
		}

		if containerId, err := docker.CreateContainer(containerConfig, appName, nil); err != nil {
			log.Println(err)
		} else {
			config := dockerclient.HostConfig{PortBindings:makeBinding("8888", "8888")}
			if err := docker.StartContainer(containerId, &config); err != nil {
				log.Printf("cannot start container: %s", err)
			} else {
				log.Printf("Start container success: %s", containerId)
				appDefault.ContainerId = containerId[0:12]
				if err = manager.Apps.CreateOrUpdate(&appDefault); err != nil {
					log.Println(err)
				}
			}
		}
	}

	manager.CliClient.UpdateState(manager.Device.Config.ApplicationId, manager.Device.Config.DeviceId, DeviceUpdateFinish)
}

// Get container application on device
func (app *App) GetContainerId(appName, dockerSocket string) (string, error) {
	var containerId string
	if docker, err := dockerclient.NewDockerClient("unix://" + dockerSocket, nil); err != nil {
		return "", err
	} else if containers, err := docker.ListContainers(false, false, "{\"name\":[\"" + appName + "\"]}"); err != nil {
		log.Printf("cannot get container: %s", err)
	} else if containerInfo, err := docker.InspectContainer(containers[0].Id); err != nil {
		return "", err
	} else {
		containerId = containerInfo.Id[0:12]
	}
	return containerId, nil
}

// TODO: use dockerclient to kill an app
func (app *App) Kill(dockerSocket string) (err error) {
	log.Printf("Killing app %d - %s", app.AppId, app.ContainerId)
	if docker, err := dockerclient.NewDockerClient("unix://" + dockerSocket, nil); err != nil {
		return err
	} else {
		if err := docker.KillContainer(app.ContainerId, ""); err != nil {
			//log.Printf("cannot kill container: %s", err)
			return err
		} else {
			log.Printf("Kill container success: %s", app.ContainerId)
			return nil
		}
	}
	return
}

func makeBinding(ip, port string) map[string][]dockerclient.PortBinding {
	return map[string][]dockerclient.PortBinding{
		fmt.Sprintf("%s/tcp", port): {
			{
				HostIp:   ip,
				HostPort: port,
			},
		},
	}
}

// TODO: use dockerclient to start an app
// TODO: implement logging
// TODO: implement web terminal
func (app *App) Start(dockerSocket string) (err error) {
	log.Printf("Starting app %d", app.AppId)

	if docker, err := dockerclient.NewDockerClient("unix://" + dockerSocket, nil); err != nil {
		return err
	} else {
		config := dockerclient.HostConfig{PortBindings:makeBinding("80", "80")}
		if err := docker.StartContainer(app.ContainerId, &config); err != nil {
			//log.Printf("cannot start container: %s", err)
			return err
		} else {
			log.Printf("Start container success: %s", app.ContainerId)
			return nil
		}
	}
	return
}

func (app *App) Stop(dockerSocket string) (err error) {
	log.Printf("Stopping app %d", app.AppId)

	if docker, err := dockerclient.NewDockerClient("unix://" + dockerSocket, nil); err != nil {
		return err
	} else {
		if err = docker.StopContainer(app.ContainerId, 5); err != nil {
			//log.Printf("cannot stop container: %s", err)
			return err
		} else {
			log.Printf("Stop container success: %s", app.ContainerId)
			return nil
		}
	}
	return
}

// TODO: use dockerclient or deltas to fetch an app image
func (app *App) Fetch(dockerSocket string) (err error) {
	log.Println("pull image")

	authConfig := dockerclient.AuthConfig{Username:"chanhlv93", Password:"chanhlove1993"}
	if docker, err := dockerclient.NewDockerClient("unix://" + dockerSocket, nil); err != nil {
		return err
	} else {
		if err = docker.PullImage("chanhlv93/cli-app", &authConfig); err != nil {
			log.Printf("cannot pull image: %s", err)
		} else {
			log.Println("Pull image success!")
		}
	}

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
		if lock, err := os.OpenFile(path, os.O_WRONLY | os.O_CREATE | os.O_EXCL, 0777); err != nil {
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
