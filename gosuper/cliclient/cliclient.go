package cliclient

import (
	"log"
	"time"
	"encoding/json"
	"github.com/go-resty/resty"
	"github.com/resin-io/resin-supervisor/gosuper/supermodels"

	"fmt"
)

type Client struct {
	BaseApiEndpoint string
	ApiKey 		string
}

type DeviceRegister struct {
	Id 		int		`json:"Id,omitempty"`
	Name 		string 		`json:"name"`
	Appid 		string 		`json:"appid"`
	Uuid 		string 		`json:"uuid"`
	Devicetype 	string 		`json:"devicetype"`
}

func (client *Client) Getapplication() (apps []supermodels.App, err error) {
	resp, err := resty.R().
		//SetQueryString("apikey=" + client.ApiKey).
		SetHeader("Accept", "application/json").
		Get(client.BaseApiEndpoint + "/v1/app")
	if err != nil {
		log.Println(err)
	}
	log.Println(resp.Body())
	if err := json.Unmarshal(resp.Body(), &apps); err != nil {
		log.Println(err)
	}

	return
}

func (client *Client) RegisterDevice(devRegister DeviceRegister) (registeredAt float64, deviceId int, err error) {
	fmt.Printf("devRegister = ", devRegister)
	resp, err := resty.R().
		SetQueryString("apikey=" + client.ApiKey).
		SetHeader("Content-Type", "application/json").
		/*SetHeader("Accept", "application/json").*/
		SetBody(devRegister).

		Post(client.BaseApiEndpoint + "/v1/device")

	if err != nil {
		log.Println(err)
	}

	var deviceRegistered DeviceRegister
	registeredAtFloat64 := float64(time.Now().Unix())
	registeredAt = registeredAtFloat64

	if err := json.Unmarshal(resp.Body(), &deviceRegistered); err != nil {
		log.Println(err)
	}

	deviceId = deviceRegistered.Id

	return
}
