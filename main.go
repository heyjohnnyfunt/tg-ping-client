package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Arman92/go-tdlib"
)

type Proxy struct {
	Type  string `json:"type`
	IP    string `json:"ip"`
	Port  int32  `json:"port"`
	Login string `json:"login"`
	Pass  string `json:"pass"`
}

type Config struct {
	APIID   string `json:"APIID"`
	APIHash string `json:"APIHash"`
	Proxy   Proxy  `json:"proxy"`
}

func LoadConfiguration(file string) Config {
	var config Config
	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		fmt.Println(err.Error())
	}
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)
	return config
}

var allChats []*tdlib.Chat
var haveFullChatList bool

// see https://stackoverflow.com/questions/37782348/how-to-use-getchats-in-tdlib
func getChatList(client *tdlib.Client, limit int) error {

	if !haveFullChatList && limit > len(allChats) {
		offsetOrder := int64(math.MaxInt64)
		offsetChatID := int64(0)
		var lastChat *tdlib.Chat

		if len(allChats) > 0 {
			lastChat = allChats[len(allChats)-1]
			offsetOrder = int64(lastChat.Order)
			offsetChatID = lastChat.ID
		}

		// get chats (ids) from tdlib
		chats, err := client.GetChats(tdlib.JSONInt64(offsetOrder),
			offsetChatID, int32(limit-len(allChats)))
		if err != nil {
			return err
		}
		if len(chats.ChatIDs) == 0 {
			haveFullChatList = true
			return nil
		}

		for _, chatID := range chats.ChatIDs {
			// get chat info from tdlib
			chat, err := client.GetChat(chatID)
			if err == nil {
				allChats = append(allChats, chat)
			} else {
				return err
			}
		}
		return getChatList(client, limit)
	}
	return nil
}

func main() {

	var chatID int64
	var proxyAssigned *tdlib.Proxy
	var err error

	tdlib.SetLogVerbosityLevel(1)
	tdlib.SetFilePath("./errors.txt")

	config := LoadConfiguration("./config.json")
	chatID = 860175318

	// Create new instance of client
	client := tdlib.NewClient(tdlib.Config{
		APIID:               config.APIID,
		APIHash:             config.APIHash,
		SystemLanguageCode:  "en",
		DeviceModel:         "Server",
		SystemVersion:       "1.0.0",
		ApplicationVersion:  "1.0.0",
		UseMessageDatabase:  true,
		UseFileDatabase:     true,
		UseChatInfoDatabase: true,
		UseTestDataCenter:   false,
		DatabaseDirectory:   "./tdlib-db",
		FileDirectory:       "./tdlib-files",
		IgnoreFileNames:     false,
	})

	/* proxies, err := client.GetProxies()
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(proxies)
	}
	*/
	// You can set username and password to empty of don't need it
	switch config.Proxy.Type {

	case "socks5":
		// Socks5
		proxyAssigned, err = client.AddProxy(config.Proxy.IP, config.Proxy.Port, true, tdlib.NewProxyTypeSocks5(config.Proxy.Login, config.Proxy.Pass))
		fmt.Println("SOCKS proxy")

	case "http":
		// HTTP - HTTPS proxy
		proxyAssigned, err = client.AddProxy(config.Proxy.IP, config.Proxy.Port, true, tdlib.NewProxyTypeHttp(config.Proxy.Login, config.Proxy.Pass, false))
		fmt.Println("HTTP proxy")

	case "mtproto":
		// MtProto Proxy
		proxyAssigned, err = client.AddProxy(config.Proxy.IP, config.Proxy.Port, true, tdlib.NewProxyTypeMtproto(config.Proxy.Login))
		fmt.Println("MTPROTO proxy")

	default:
		fmt.Println("No proxy")

	}

	if err == nil {
		seconds, _ := client.PingProxy(proxyAssigned.ID)
		fmt.Printf("Proxy ping: %f s \n", seconds.Seconds)
	} else {
		fmt.Println(err)
	}

	// Handle Ctrl+C
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		client.DestroyInstance()
		os.Exit(1)
	}()

	for {
		currentState, _ := client.Authorize()
		if currentState.GetAuthorizationStateEnum() == tdlib.AuthorizationStateWaitPhoneNumberType {
			fmt.Print("Enter phone: ")
			var number string
			fmt.Scanln(&number)
			_, err := client.SendPhoneNumber(number)
			if err != nil {
				fmt.Printf("Error sending phone number: %v", err)
			}
		} else if currentState.GetAuthorizationStateEnum() == tdlib.AuthorizationStateWaitCodeType {
			fmt.Print("Enter code: ")
			var code string
			fmt.Scanln(&code)
			_, err := client.SendAuthCode(code)
			if err != nil {
				fmt.Printf("Error sending auth code : %v", err)
			}
		} else if currentState.GetAuthorizationStateEnum() == tdlib.AuthorizationStateWaitPasswordType {
			fmt.Print("Enter Password: ")
			var password string
			fmt.Scanln(&password)
			_, err := client.SendAuthPassword(password)
			if err != nil {
				fmt.Printf("Error sending auth password: %v", err)
			}
		} else if currentState.GetAuthorizationStateEnum() == tdlib.AuthorizationStateReadyType {
			fmt.Println("Authorization Ready! Let's rock")
			break
		}
	}

	// Main loop

	var pingTime time.Time

	go func() {
		for {
			var message string
			// fmt.Print("Enter message:")
			fmt.Scanln(&message)

			switch message {

			case "ping":
				pingTime = time.Now()
				// chatID := int64(860175318) // tg-ping-bot chat id

				inputMsgTxt := tdlib.NewInputMessageText(tdlib.NewFormattedText(message, nil), true, true)
				_, err := client.SendMessage(chatID, 0, false, true, nil, inputMsgTxt)
				if err != nil {
					fmt.Printf("Error while sending: %s", err)
					continue
				}
			case "chatlist":
				// get at most 1000 chats list
				getChatList(client, 1000)
				fmt.Printf("got %d chats\n", len(allChats))

				for _, chat := range allChats {
					fmt.Printf("Chat title: %s\tChat id: %d\n", chat.Title, int64(chat.ID))
				}

			case "setchat":
				var chatStrId string
				var err error
				fmt.Print("Enter chat ID:")
				fmt.Scanln(&chatStrId)
				// get at most 1000 chats list
				chatID, err = strconv.ParseInt(chatStrId, 10, 64)
				if err != nil {
					fmt.Println("Invalid int64 value")
				}

			}

			// fmt.Println(result)
		}

	}()

	// rawUpdates gets all updates comming from tdlib
	rawUpdates := client.GetRawUpdatesChannel(100)
	for update := range rawUpdates {
		// Show all updates

		updatedChatID := update.Data["chat_id"]
		if updatedChatID != nil {
			chatID2 := updatedChatID.(float64)
			if int64(chatID2) == chatID {

				// fmt.Println(int64(chatID2))
				if update.Data["last_message"] != nil {
					reponseText := update.Data["last_message"].(map[string]interface{})["content"].(map[string]interface{})["text"].(map[string]interface{})["text"].(string)
					if strings.Contains(reponseText, "pong") {
						if !pingTime.IsZero() {
							elapsed := time.Since(pingTime)
							fmt.Printf("pong time: %s\n", elapsed)
							pingTime = time.Time{}
						}
						// fmt.Println(reponseText)

					}

				}
			}
		}
	}

}
