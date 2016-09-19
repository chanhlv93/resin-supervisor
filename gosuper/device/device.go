package device

// TODO: implement function to get OS version
// TODO: implement ApplyBootConfig

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/resin-io/resin-supervisor/gosuper/application/updatestatus"
	"github.com/resin-io/resin-supervisor/gosuper/config"
	"github.com/resin-io/resin-supervisor/gosuper/resin"
	"github.com/resin-io/resin-supervisor/gosuper/cliclient"
	"github.com/resin-io/resin-supervisor/gosuper/supermodels"
	"github.com/resin-io/resin-supervisor/gosuper/utils"
	"fmt"
)

const uuidByteLength = 31
const preloadedAppsPath = "/tmp/agent/apps.json"

//Add cli-client to interacting with web service
type Device struct {
	Id            int
	Uuid          string
	Bootstrapped  bool
	waitChannels  []chan bool
	bootstrapLock sync.Mutex
	Config        config.UserConfig
	DbConfig      *supermodels.Config
	ResinClient   *resin.Client
	CliClient     cliclient.Client
	SuperConfig   config.SupervisorConfig
	UpdateStatus  updatestatus.UpdateStatus
}

func (dev Device) readConfigAndEnsureUuid() (uuid string, conf config.UserConfig, err error) {
	if conf, err = config.ReadConfig(config.DefaultConfigPath); err != nil {
	} else if conf.Uuid != "" {
		uuid = conf.Uuid
	} else if uuid, err = utils.RandomHexString(uuidByteLength); err != nil {
		conf.Uuid = uuid
		err = config.WriteConfig(conf, config.DefaultConfigPath)
	}

	if err != nil {
		time.Sleep(time.Duration(dev.SuperConfig.BootstrapRetryDelay) * time.Millisecond)
		return dev.readConfigAndEnsureUuid()
	}

	return
}

// This should be moved to application or supermodels?
func loadPreloadedApps(appsCollection *supermodels.AppsCollection, dockerSocket string, appid int) {
	log.Println("------------------> START LOAD PREPARE APP")
	/*var err error
	// Create app default in agent database
	appDefault := application.App{AppId: appid, ContainerId:"", Commit:"", Env:nil, ImageId:"chanhlv93/cli-app"}
	if err = appsCollection.CreateOrUpdate(&appDefault); err != nil {
		log.Println(err)
		return
	}
	//Run default container app
	application.StartDefaultApp(dockerSocket)

	if containerIdUpdate, err := appDefault.GetContainerId("cli-app", dockerSocket); err != nil {
		log.Println(err)
	} else {
		appDefault.ContainerId = containerIdUpdate
		if err = appsCollection.CreateOrUpdate(&appDefault); err != nil {
			log.Println(err)
		}
	}*/

	log.Println("------------------> FINISHED LOAD PREPARE APP")
}

// TODO use dev.ResinClient.RegisterDevice
func (dev *Device) register() (registeredAt int, deviceId int, err error) {
	return
}

func (dev *Device) bootstrap() (err error) {
	if dev.Config.DeviceType == "" {
		dev.Config.DeviceType = "raspberry-pi"
	}
	if dev.Config.RegisteredAt == 0 {
		if registeredAt, deviceId, err := dev.register(); err == nil {
			dev.Config.RegisteredAt = registeredAt
			dev.Config.DeviceId = deviceId
			if err = config.WriteConfig(dev.Config, config.DefaultConfigPath); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	//log.Println("===>>> Save to db")
	err = config.SaveToDB(dev.Config, dev.DbConfig)
	return
}

func (dev *Device) BootstrapOrRetry() {
	//utils.MixpanelTrack("Device bootstrap", nil)
	if err := dev.bootstrap(); err != nil {
		log.Printf("Device bootstrap failed, retrying: %s", err)
		time.AfterFunc(time.Duration(dev.SuperConfig.BootstrapRetryDelay)*time.Millisecond, dev.BootstrapOrRetry)
	}
}

func New(appsCollection *supermodels.AppsCollection, dbConfig *supermodels.Config, superConfig config.SupervisorConfig) (dev *Device, err error) {
	device := Device{}
	var uuid string
	var conf config.UserConfig

	device.DbConfig = dbConfig
	device.SuperConfig = superConfig
	//log.Printf(dbConfig)
	if uuid, err = dbConfig.Get("uuid"); err != nil {
	} else if uuid != "" {
		log.Printf("Found registered device with uuid: " + uuid)
		if apikey, err := dbConfig.Get("apiKey"); err == nil {
			device.Uuid = uuid
			device.CliClient = cliclient.Client{BaseApiEndpoint: device.SuperConfig.ApiEndpoint, ApiKey:apikey}

			//device.ResinClient = resin.NewClient(superConfig.ApiEndpoint, apikey)
			device.FinishBootstrapping()
			dev = &device
		} else {
			// This should *never* happen
			log.Fatalf("Device is bootstrapped, but could not get apikey from DB: %s", err)
		}
	} else {
		log.Printf("New device detected, bootstrapping...")
		if uuid, conf, err = device.readConfigAndEnsureUuid(); err == nil {
			//device.UpdateStatus()
			device.Uuid = uuid
			device.Config = conf

			device.Config.Uuid = uuid
			//device.Config.DeviceType = conf.DeviceType

			//device.ResinClient = resin.NewClient(superConfig.ApiEndpoint, conf.ApiKey)
			deviceRegister := cliclient.DeviveRegister{Appid: conf.ApplicationId, Name: conf.ApplicationName, Uuid: uuid, Devicetype: conf.DeviceType}
			//Set up cli client with needed informations to call other func.
			device.CliClient = cliclient.Client{BaseApiEndpoint: device.SuperConfig.ApiEndpoint, ApiKey:conf.ApiKey}

			regAt, deviceId, errReg := device.CliClient.RegisterDevice(deviceRegister)
			if errReg != nil {
				log.Println(errReg)
			}

			//Update device id into config.json file after register success
			device.Config.DeviceId = deviceId
			conf.DeviceId = deviceId
			device.Config.RegisteredAt = regAt
			err = config.WriteConfig(device.Config, config.DefaultConfigPath)

			device.BootstrapOrRetry()
			dev = &device
		}
	}
	return
}

func (dev *Device) GetId() (id int, err error) {
	if dev.Id != 0 {
		return dev.Id, nil
	}
	remoteDev, err := dev.ResinClient.GetDevice(dev.Uuid)
	if err != nil {
		var ok bool
		if id, ok = remoteDev["id"].(int); !ok {
			err = errors.New("Invalid id received from API")
		} else {
			dev.Id = id
		}
	}
	return id, err
}

func (dev Device) WaitForBootstrap() {
	dev.bootstrapLock.Lock()
	if dev.Bootstrapped {
		dev.bootstrapLock.Unlock()
	} else {
		dev.waitChannels = append(dev.waitChannels, make(chan bool))
		dev.bootstrapLock.Unlock()
		<-dev.waitChannels[len(dev.waitChannels)]
	}
}

func (dev Device) FinishBootstrapping() {
	dev.bootstrapLock.Lock()
	dev.Bootstrapped = true
	for _, c := range dev.waitChannels {
		c <- true
	}
	dev.bootstrapLock.Unlock()
}

// TODO: implement UpdateState (using dev.ResinClient.UpdateDevice)
func (dev *Device) UpdateState(m map[string]interface{}) {

}