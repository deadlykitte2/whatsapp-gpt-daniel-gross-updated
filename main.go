package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

type MyClient struct {
	WAClient       *whatsmeow.Client
	eventHandlerID uint32
}

func (mycli *MyClient) register() {
	mycli.eventHandlerID = mycli.WAClient.AddEventHandler(mycli.eventHandler)
}

func (mycli *MyClient) reconnectHandler() {
	for {
		if !mycli.WAClient.IsConnected() {
			err := mycli.WAClient.Connect()
			if err != nil {
				fmt.Printf("Reconnection error: %v\n", err)
				time.Sleep(5 * time.Second)
			} else {
				fmt.Println("Reconnected successfully")
			}
		}
		time.Sleep(10 * time.Second)
	}
}

func (mycli *MyClient) eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		// Ignore group messages
		if v.Info.Chat.Server == "g.us" {
			return
		}
		
		newMessage := v.Message
		msg := newMessage.GetConversation()
		fmt.Printf("Message from %s: %s\n", v.Info.Sender.User, msg)
		
		if msg == "" {
			return
		}
		// Make request to ChatGPT server with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		urlEncoded := url.QueryEscape(msg)
		reqURL := fmt.Sprintf("http://localhost:5001/chat?q=%s", urlEncoded)
		
		req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		if err != nil {
			fmt.Printf("Error creating request: %v\n", err)
			return
		}
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Error making request: %v\n", err)
			return
		}
		defer resp.Body.Close()
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(resp.Body); err != nil {
			fmt.Printf("Error reading response: %v\n", err)
			return
		}
		newMsg := buf.String()
		response := &waProto.Message{Conversation: proto.String(newMsg)}
		userJid := types.NewJID(v.Info.Sender.User, types.DefaultUserServer)
		if _, err := mycli.WAClient.SendMessage(ctx, userJid, response); err != nil {
			fmt.Printf("Error sending message: %v\n", err)
		}
	}
}

func main() {
	dbLog := waLog.Stdout("Database", "DEBUG", true)
	container, err := sqlstore.New("sqlite3", "file:examplestore.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}
	
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}
	
	clientLog := waLog.Stdout("Client", "DEBUG", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	
	// Remove SetAutoReconnect as it doesn't exist
	
	mycli := &MyClient{WAClient: client}
	mycli.register()
	
	if client.Store.ID == nil {
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			panic(err)
		}
		
		for evt := range qrChan {
			if evt.Event == "code" {
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		err = client.Connect()
		if err != nil {
			panic(err)
		}
	}
	
	// Add reconnection handler
	go mycli.reconnectHandler()
	
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	
	client.Disconnect()
}