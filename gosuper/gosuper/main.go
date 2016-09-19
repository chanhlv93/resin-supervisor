package main

import (
	"log"
	"time"
	"math/rand"

	"github.com/resin-io/resin-supervisor/gosuper/application"
	"github.com/resin-io/resin-supervisor/gosuper/config"
	"github.com/resin-io/resin-supervisor/gosuper/device"
	"github.com/resin-io/resin-supervisor/gosuper/psutils"
	"github.com/resin-io/resin-supervisor/gosuper/resin"
	"github.com/resin-io/resin-supervisor/gosuper/supermodels"
	"github.com/resin-io/resin-supervisor/gosuper/cliclient"
)

var ResinDataPath string = "/mnt/root/resin-data/"

type Supervisor struct {
	Config             config.SupervisorConfig
	AppsCollection     *supermodels.AppsCollection
	DbConfig           *supermodels.Config
	ResinClient        *resin.Client
	Device             *device.Device
	ApplicationManager *application.Manager
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// TODO: implement connectivityCheck
func connectivityCheck() {

}

func testConnectivityCheck(config config.SupervisorConfig) {
	cliClient := cliclient.Client{BaseApiEndpoint: config.ApiEndpoint, ApiKey:""}
	for {
		check, err := cliClient.CheckConnectivity(); if err != nil {
			log.Println(err)
		}
		log.Println("check connectivity result", check)
		time.Sleep(time.Second * 10)
	}

}

// TODO: move to utils?
func startOOMProtectionTimer(hostproc string, dockerSocket string) *time.Ticker {
	ticker := time.NewTicker(time.Minute * 5) //Timer runs every 5 minutes
	procs := &psutils.Procs{hostproc}
	log.Println("Changing oom_score_adj for the supervisor container to -800")
	if err := procs.AdjustDockerOOMPriority("unix://"+dockerSocket, "cli-app", -800, false); err != nil {
		log.Printf("FAILED to OOM protect supervisor container: %s\n", err)
	}
	// Code below this could be eventually deprecated after all the devices are > 5 Jan 2016 deployment as this will be handled in the HOST OS.
	log.Println("Changing oom_score_adj for openvpn and connmand to -1000 if 0, every 5 minutes")
	// Errors are not being caught here as users could have openvpn and connmand disabled.
	procs.AdjustOOMPriorityByName("openvpn", -1000, true)
	procs.AdjustOOMPriorityByName("connmand", -1000, true)
	go func() {
		for _ = range ticker.C {
			procs.AdjustOOMPriorityByName("openvpn", -1000, true)
			procs.AdjustOOMPriorityByName("connmand", -1000, true)
		}
	}()
	return ticker
}

func (supervisor *Supervisor) Start(connectivityCheckEnabled bool, oomProtectionEnabled bool) {
	var err error

	if connectivityCheckEnabled {
		go connectivityCheck()
		//go testConnectivityCheck(supervisor.Config)
	}

	supervisor.Config = config.GetSupervisorConfig()

	// Disable VPN connection
	if oomProtectionEnabled {
		// Start OOMProtectionTimer for protecting Openvpn/Connman
		//defer startOOMProtectionTimer(supervisor.Config.HostProc, supervisor.Config.DockerSocket).Stop()
	}

	// Disable Mixpanel
	//if err = utils.MixpanelInit(supervisor.Config.MixpanelToken); err != nil {
	//	log.Printf("Failed to initialize Mixpanel client: %s", err)
	//}

	if supervisor.AppsCollection, supervisor.DbConfig, err = supermodels.New(supervisor.Config.DatabasePath); err != nil {
		log.Fatalf("Failed to start database: %s", err)
	} else if supervisor.Device, err = device.New(supervisor.AppsCollection, supervisor.DbConfig, supervisor.Config); err != nil {
		log.Fatalf("Failed to start device bootstrapping: %s", err)
	} else {
		//utils.MixpanelSetId(supervisor.Device.Uuid)
		//supervisor.ResinClient = supervisor.Device.ResinClient
		if supervisor.ApplicationManager, err = application.NewManager(supervisor.AppsCollection, supervisor.DbConfig, supervisor.Device, supervisor.Config); err != nil {
			log.Fatalf("Failed to initialize applications manager: %s", err)
		} else {
			//supervisor.Device.WaitForBootstrap()
			// TODO: apikey and log channel generation
			StartApi(supervisor.Config.ListenPort, supervisor.ApplicationManager)
			// TODO: Update device state
			// TODO: IP address update interval
		}
	}
}

func waitForever() {
	c := make(chan bool)
	<-c
}

func main() {
	var supervisor Supervisor

	log.SetFlags(log.Lshortfile | log.LstdFlags)
	log.Println("Resin Supervisor starting")

	supervisor.Start(true, true)
	waitForever()
}
