package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dennwc/dom/js"
	"github.com/zorchenhimer/MovieNight/common"
)

var (
	auth  common.CommandLevel
	color string
)

func recieve(v []js.Value) {
	if len(v) == 0 {
		fmt.Println("No data received")
		return
	}

	chatJSON, err := common.DecodeData(v[0].String())
	if err != nil {
		fmt.Printf("Error decoding data: %s\n", err)
		js.Call("appendMessages", v)
		return
	}

	chat, err := chatJSON.ToData()
	if err != nil {
		fmt.Printf("Error converting ChatDataJSON to ChatData of type %d: %v", chatJSON.Type, err)
	}

	switch chat.Type {
	case common.DTHidden:
		h := chat.Data.(common.HiddenMessage)
		switch h.Type {
		case common.CdUsers:
			names = nil
			for _, i := range h.Data.([]interface{}) {
				names = append(names, i.(string))
			}
		case common.CdAuth:
			auth = h.Data.(common.CommandLevel)
		case common.CdColor:
			color = h.Data.(string)
			js.Get("document").Set("cookie", fmt.Sprintf("color=%s;", color))
		}
	case common.DTEvent:
		d := chat.Data.(common.DataEvent)
		// A server message is the only event that doesn't deal with names.
		if d.Event != common.EvServerMessage {
			websocketSend("", common.CdUsers)
		}
		// on join or leave, update list of possible user names
		fallthrough
	case common.DTChat:
		js.Call("appendMessages", chat.Data.HTML())
	case common.DTCommand:
		d := chat.Data.(common.DataCommand)

		switch d.Command {
		case common.CmdPlaying:
			if d.Arguments == nil || len(d.Arguments) == 0 {
				js.Call("setPlaying", "", "")

			} else if len(d.Arguments) == 1 {
				js.Call("setPlaying", d.Arguments[0], "")

			} else if len(d.Arguments) == 2 {
				js.Call("setPlaying", d.Arguments[0], d.Arguments[1])
			}
		case common.CmdRefreshPlayer:
			js.Call("initPlayer", nil)
		case common.CmdPurgeChat:
			js.Call("purgeChat", nil)
			js.Call("appendMessages", d.HTML())
		case common.CmdHelp:
			url := "/help"
			if d.Arguments != nil && len(d.Arguments) > 0 {
				url = d.Arguments[0]
			}
			js.Call("appendMessages", d.HTML())
			js.Get("window").Call("open", url, "_blank", "menubar=0,status=0,toolbar=0,width=300,height=600")
		}
	}
}

func websocketSend(msg string, dataType common.ClientDataType) error {
	if strings.TrimSpace(msg) == "" && dataType == common.CdMessage {
		return nil
	}

	data, err := json.Marshal(common.ClientData{
		Type:    dataType,
		Message: msg,
	})
	if err != nil {
		return fmt.Errorf("could not marshal data: %v", err)
	}

	js.Call("websocketSend", string(data))
	return nil
}

func send(this js.Value, v []js.Value) interface{} {
	if len(v) != 1 {
		showChatError(fmt.Errorf("expected 1 parameter, got %d", len(v)))
		return false
	}

	err := websocketSend(v[0].String(), common.CdMessage)
	if err != nil {
		showChatError(err)
		return false
	}
	return true
}

func showChatError(err error) {
	if err != nil {
		fmt.Printf("Could not send: %v\n", err)
		js.Call("appendMessages", `<div><span style="color: red;">Could not send message</span></div>`)
	}
}

func isValidColor(this js.Value, v []js.Value) interface{} {
	if len(v) != 1 {
		return false
	}
	return common.IsValidColor(v[0].String())
}

func isValidName(this js.Value, v []js.Value) interface{} {
	if len(v) != 1 {
		return false
	}
	return common.IsValidName(v[0].String())
}

func debugValues(v []js.Value) {
	fmt.Printf("auth %#v\n", auth)
	fmt.Printf("color %#v\n", color)
	fmt.Printf("currentName %#v\n", currentName)
	fmt.Printf("names %#v\n", names)
	fmt.Printf("filteredNames %#v\n", filteredNames)
}

func main() {
	js.Set("processMessageKey", js.FuncOf(processMessageKey))
	js.Set("sendMessage", js.FuncOf(send))
	js.Set("isValidColor", js.FuncOf(isValidColor))
	js.Set("isValidName", js.FuncOf(isValidName))

	js.Set("recieveMessage", js.CallbackOf(recieve))
	js.Set("processMessage", js.CallbackOf(processMessage))
	js.Set("debugValues", js.CallbackOf(debugValues))

	// This is needed so the goroutine does not end
	for {
		// heatbeat to keep connection alive to deal with nginx
		if js.Get("inChat").Bool() {
			websocketSend("", common.CdPing)
		}
		time.Sleep(time.Second * 10)
	}
}
