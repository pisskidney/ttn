// Copyright © 2015 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package pubsub

import (
	"bytes"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/TheThingsNetwork/ttn/core"
	httpadapter "github.com/TheThingsNetwork/ttn/core/adapters/http"
	"github.com/TheThingsNetwork/ttn/core/adapters/http/parser"
	"github.com/TheThingsNetwork/ttn/utils/errors"
	. "github.com/TheThingsNetwork/ttn/utils/testing"
	"github.com/brocaar/lorawan"
)

// NextRegistration() (core.Registration, core.AckNacker, error)
func TestNextRegistration(t *testing.T) {
	tests := []struct {
		AppId      string
		AppUrl     string
		DevAddr    string
		NwkSKey    string
		WantResult *core.Registration
		WantError  *string
	}{
		// Valid device address
		{
			AppId:   "appid",
			AppUrl:  "myhandler.com:3000",
			NwkSKey: "000102030405060708090a0b0c0d0e0f",
			DevAddr: "14aab0a4",
			WantResult: &core.Registration{
				DevAddr:   lorawan.DevAddr([4]byte{0x14, 0xaa, 0xb0, 0xa4}),
				Recipient: core.Recipient{Id: "appid", Address: "myhandler.com:3000"},
				Options:   lorawan.AES128Key([16]byte{0x0, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf}),
			},
			WantError: nil,
		},
		// Invalid device address
		{
			AppId:      "appid",
			AppUrl:     "myhandler.com:3000",
			NwkSKey:    "000102030405060708090a0b0c0d0e0f",
			DevAddr:    "INVALID",
			WantResult: nil,
			WantError:  nil,
		},
		// Invalid NwkSKey address
		{
			AppId:      "appid",
			AppUrl:     "myhandler.com:3000",
			NwkSKey:    "00112233445566778899af",
			DevAddr:    "14aab0a4",
			WantResult: nil,
			WantError:  nil,
		},
	}

	// Logging
	ctx := GetLogger(t, "Adapter")
	a, err := httpadapter.NewAdapter(3021, parser.JSON{}, ctx)
	if err != nil {
		panic(err)
	}

	adapter, err := NewAdapter(a, parser.PubSub{}, ctx)
	client := &client{adapter: "0.0.0.0:3021"}
	if err != nil {
		panic(err)
	}

	for _, test := range tests {
		// Describe
		Desc(t, "Trying to register %s -> %s, %s, %s", test.DevAddr, test.AppId, test.AppUrl, test.NwkSKey)
		<-time.After(time.Millisecond * 100)

		// Build
		gotErr := make(chan error)
		gotConf := make(chan core.Registration)
		go client.send(test.AppId, test.AppUrl, test.DevAddr, test.NwkSKey)

		// Operate
		go func() {
			config, _, err := adapter.NextRegistration()
			gotErr <- err
			gotConf <- config
		}()

		// Check
		select {
		case err := <-gotErr:
			checkErrors(t, test.WantError, err)
		case <-time.After(time.Millisecond * 250):
			checkErrors(t, test.WantError, nil)
		}

		select {
		case conf := <-gotConf:
			checkRegistrationResult(t, test.WantResult, &conf)
		case <-time.After(time.Millisecond * 250):
			checkRegistrationResult(t, test.WantResult, nil)
		}
	}
}

func checkErrors(t *testing.T, want *string, got error) {
	if want == nil && got == nil || got.(errors.Failure).Nature == *want {
		Ok(t, "Check errors")
		return
	}

	Ko(t, "Expected error to be %s but got %v", want, got)
}

func checkRegistrationResult(t *testing.T, want, got *core.Registration) {
	if !reflect.DeepEqual(want, got) {
		Ko(t, "Received configuration doesn't match expectations")
		return
	}

	Ok(t, "Check registration result")
}

// Operate utilities

// Wrapper around the http client
type client struct {
	http.Client
	adapter string
}

// send is a convinient helper to send HTTP from a handler to the adapter
func (c *client) send(appId, appUrl, devAddr, nwkSKey string) http.Response {
	buf := new(bytes.Buffer)
	if _, err := buf.WriteString(fmt.Sprintf(`{"app_id":"%s","app_url":"%s","nwks_key":"%s"}`, appId, appUrl, nwkSKey)); err != nil {
		panic(err)
	}
	request, err := http.NewRequest("PUT", fmt.Sprintf("http://%s/end-devices/%s", c.adapter, devAddr), buf)
	if err != nil {
		panic(err)
	}
	request.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(request)
	if err != nil {
		panic(err)
	}
	return *resp
}
