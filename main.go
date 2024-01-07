package main

import (
	"context"
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Chrome --proxy-server=http://127.0.0.1:7890 --start-maximized --new-window https://www.google.com

var (
	queue = make([]*ChanMessage, 0)
	mu    sync.Mutex
	cmd   string
	proxy string
	sock  = "127.0.0.1:8080"
)

var udr = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Hub struct {
	*websocket.Conn
	message *ChanMessage
	Closed  bool
	Token   string
}

type ChanMessage struct {
	sync.Mutex
	cookies []http.Cookie
	expired bool
	message chan string
	h       *Hub
}

func init() {
	_ = godotenv.Load()
	cmd = LoadEnvVar("CMD", cmd)
	proxy = LoadEnvVar("PROXY", proxy)
	sock = LoadEnvVar("SOCK", sock)
}

func LoadEnvVar(key, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		value = defaultValue
	}
	return value
}

func (h *Hub) Init() {
	log.Printf("[%s] Add connection!", h.Token)
	go func() {
		for {
			if h.Closed == true {
				return
			}
			if ReadMessage(h) != nil {
				return
			}
			time.Sleep(time.Second)
		}
	}()

	go func() {
		for {
			if h.Closed == true {
				return
			}
			if WriteMessage(h) != nil {
				return
			}
			time.Sleep(time.Second)
		}
	}()
}

func home(w http.ResponseWriter, r *http.Request) {
	data, _ := httputil.DumpRequest(r, true)
	log.Println(string(data))

	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, "home.html")
}

func verify(w http.ResponseWriter, r *http.Request) {
	data, err := httputil.DumpRequest(r, true)
	log.Println(string(data))

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err = io.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cookies := make([]http.Cookie, 0)
	err = json.Unmarshal(data, &cookies)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Println(cookies)
	cm := &ChanMessage{cookies: cookies, expired: false, message: make(chan string)}
	pushCache(cm)

	// 等待60s，超时视为失败
	timeout, cancel := context.WithTimeout(context.TODO(), 20*time.Second)
	defer cancel()

	// 每个链接必须先处理完上一个才能继续处理下一个
	cm.Lock()
	defer cm.Unlock()

	for {
		select {
		case <-timeout.Done():
			_, _ = w.Write([]byte("timeout"))
			cm.expired = true
			if cm.h != nil {
				_ = cm.h.WriteMessage(websocket.TextMessage, []byte("delete"))
			}
			return
		default:
			message, ok := <-cm.message
			if ok {
				log.Println("waiting: ", message)
				if message == "success" {
					_, _ = w.Write([]byte("success"))
					return
				}
			}
		}
	}
}

func main() {
	if cmd != "" {
		go func() {
			args := []string{"--incognito"}
			if proxy != "" {
				args = append(args, "--proxy-server="+proxy)
			}
			args = append(args, []string{
				"--start-maximized",
				"--new-window",
				"https://www.bing.com/turing/captcha/challenge#ip=" + sock,
			}...)
			if err := exec.Command(cmd, args...).Run(); err != nil {
				log.Fatal(err)
			}
		}()
	}

	http.HandleFunc("/", home)
	http.HandleFunc("/verify", verify)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := udr.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}

		h := Hub{Conn: conn, message: nil, Closed: false, Token: r.URL.RawQuery}
		h.Init()
	})

	log.Println("start: 0.0.0.0:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func ReadMessage(h *Hub) error {
	_, data, err := h.ReadMessage()
	if err != nil {
		log.Printf("[%s] %v", h.Token, err)
		log.Printf("[%s]Is closed!", h.Token)
		h.Closed = true
		_ = h.Close()
		return err
	}

	message := string(data)
	log.Printf("[%s] %s", h.Token, message)
	if h.message == nil {
		log.Printf("[%s] warn: chan message is nil!", h.Token)
		return nil
	}

	if h.message.expired {
		log.Printf("[%s] warn: chan message is expired!", h.Token)
		return nil
	}

	h.message.message <- message
	return nil
}

func WriteMessage(h *Hub) error {
	// 每个链接必须先处理完上一个才能继续处理下一个
	if h.message != nil {
		h.message.Lock()
		defer h.message.Unlock()
	}

	cm := takeCache()
	if cm == nil {
		return nil
	}

	cm.h = h
	h.message = cm
	data, _ := json.Marshal(cm.cookies)
	log.Printf("[%s] %s, %v", h.Token, "begin captcha", cm.cookies)
	err := h.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		log.Printf("[%s] %v", h.Token, err)
		log.Printf("[%s]Is closed!", h.Token)
		h.Closed = true
		_ = h.Close()
		return err
	}
	return nil
}

// 加入缓存
func pushCache(cm *ChanMessage) {
	mu.Lock()
	defer mu.Unlock()
	queue = append(queue, cm)
}

// 取出缓存
func takeCache() (cm *ChanMessage) {
	mu.Lock()
	defer mu.Unlock()
	if len(queue) == 0 {
		return
	}

	cm = queue[0]
	queue = queue[1:]
	return
}
