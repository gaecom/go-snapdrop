package main

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	ung "github.com/DillonStreator/go-unique-name-generator"
	"github.com/DillonStreator/go-unique-name-generator/dictionaries"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/ua-parser/uap-go/uaparser"
)

//go:embed regexes.yaml
var content []byte

type Name struct {
	Model       string `json:"model"`
	OS          string `json:"os"`
	Browser     string `json:"browser"`
	Type        string `json:"type"`
	DeviceName  string `json:"deviceName"`
	DisplayName string `json:"displayName"`
}
type Info struct {
	Id           string `json:"id"`
	Name         Name   `json:"name"`
	RtcSupported bool   `json:"rtcSupported"`
}

type Peer struct {
	Socket       *websocket.Conn
	Ip           string
	Id           string
	RtcSupported bool
	Name         Name
	Timer        bool
	LastBeat     time.Time
	Mux          sync.Mutex
}

func (s *Peer) Init(socket *websocket.Conn, request *http.Request) {
	s.Socket = socket
	s.SetIP(request)
	s.SetPeerId(request)
	if strings.Contains(request.URL.Path, "webrtc") {
		s.RtcSupported = true
	}
	s.SetName(request)
	s.LastBeat = time.Now()
}
func (s *Peer) SetIP(request *http.Request) {
	if request.Header.Get("x-forwarded-for") != "" {
		s.Ip = strings.TrimSpace(strings.Split(request.Header.Get("x-forwarded-for"), ",")[0])
	} else {
		s.Ip = strings.TrimSpace(strings.Split(request.RemoteAddr, ":")[0])
	}
	if s.Ip == "::1" || s.Ip == "::ffff:127.0.0.1" {
		s.Ip = "127.0.0.1"
	}
	//Add Private Network Support
	ip_str := strings.Split(s.Ip, ".")
	switch ip_str[0] {
	case "10":
		s.Ip = "10.0.0.0"
	case "172":
		num, err := strconv.Atoi(ip_str[1])
		if err == nil && num >= 16 && num <= 31 {
			s.Ip = "172." + ip_str[1] + ".0.0"
		}
	case "192":
		if ip_str[1] == "168" {
			s.Ip = "192.168.0.0"
		}
	}
}
func (s *Peer) SetPeerId(request *http.Request) {
	if request.Header.Get("Peerid") != "" {
		s.Id = strings.TrimSpace(request.Header.Get("Peerid"))
	} else {
		id, err := request.Cookie("peerid")
		if err != nil {
			return
		}
		s.Id = strings.TrimSpace(id.Value)
	}
}
func (s *Peer) ToString() string {
	return fmt.Sprintf("<Peer id=%s ip=%s rtcSupported=%v>", s.Id, s.Ip, s.RtcSupported)
}
func (s *Peer) SetName(request *http.Request) {
	deviceName := ""
	deviceType := ""
	parser, err := uaparser.NewFromBytes(content)
	if err != nil {
		log.Fatal(err)
	}
	ua := request.Header.Get("User-Agent")
	client := parser.Parse(ua)
	if client.Os.Family != "" {
		deviceName += client.Os.Family + " "
	}
	if client.Device.Family != "" {
		deviceName += client.Device.Family
	}
	if deviceName == "" {
		deviceName = "Unknown Device"
	}
	if strings.Contains(strings.ToLower(ua), "ipad") {
		deviceType = "tablet"
	} else if strings.Contains(strings.ToLower(ua), "mobile") {
		deviceType = "mobile"
	} else {
		deviceType = "desktop"
	}
	generator := ung.NewUniqueNameGenerator(
		ung.WithDictionaries(
			[][]string{
				dictionaries.Colors,
				dictionaries.Animals,
			},
		),
		ung.WithSeparator(" "),
		ung.WithStyle(ung.Capital),
	)
	displayName := generator.Generate()
	s.Name = Name{
		Model:       client.Device.Family,
		OS:          client.Os.Family,
		Browser:     client.Device.Family,
		Type:        deviceType,
		DeviceName:  deviceName,
		DisplayName: displayName,
	}
}
func (s *Peer) GetInfo() Info {
	return Info{Id: s.Id, Name: s.Name, RtcSupported: s.RtcSupported}
}

type Peers map[string]*Peer
type SnapdropServer struct {
	Rooms    map[string]Peers
	Upgrader *websocket.Upgrader
}

func (s *SnapdropServer) Init() {
	s.Upgrader = &websocket.Upgrader{}
	s.Rooms = map[string]Peers{}
}
func (s *SnapdropServer) OnHeaders(w http.ResponseWriter, r *http.Request) {
	response := http.Header{}
	_, err := r.Cookie("peerid")
	if err != nil {
		peerID, _ := uuid.NewUUID()
		r.Header.Set("Peerid", peerID.String())
		cookie := http.Cookie{
			Name:     "peerid",
			Value:    peerID.String(),
			SameSite: http.SameSiteStrictMode,
			Secure:   true,
		}
		response.Set("Set-Cookie", cookie.String())
	}
	conn, err := s.Upgrader.Upgrade(w, r, response)
	if err != nil {
		log.Println(err)
		return
	}
	peer := Peer{}
	peer.Init(conn, r)
	s.OnConnection(&peer)
}
func (s *SnapdropServer) OnConnection(peer *Peer) {
	s.JoinRoom(peer)
	go s.Timer(peer)
	name := map[string]interface{}{"displayName": peer.Name.DisplayName, "deviceName": peer.Name.DeviceName}
	message := map[string]interface{}{"type": "display-name", "message": name}
	s.Send(peer, message)
	s.OnMessage(peer)
}
func (s *SnapdropServer) OnMessage(peer *Peer) {
	for {
		message := map[string]interface{}{}
		err := peer.Socket.ReadJSON(&message)
		if err != nil {
			break
		}
		if message["type"] != nil {
			switch message["type"] {
			case "disconnect":
				s.LeaveRoom(peer)
			case "pong":
				peer.LastBeat = time.Now()
			}
			if message["to"] != nil && s.Rooms[peer.Ip] != nil {
				if recipientId, ok := message["to"].(string); ok {
					recipient := s.Rooms[peer.Ip][recipientId]
					delete(message, "to")
					message["sender"] = peer.Id
					s.Send(recipient, message)
				}
			}
		}
	}
}
func (s *SnapdropServer) Send(peer *Peer, message interface{}) {
	if peer == nil {
		return
	}
	peer.Mux.Lock()
	peer.Socket.WriteJSON(message)
	peer.Mux.Unlock()
}
func (s *SnapdropServer) JoinRoom(peer *Peer) {
	if s.Rooms[peer.Ip] == nil {
		s.Rooms[peer.Ip] = Peers{}
	}
	for _, otherPeer := range s.Rooms[peer.Ip] {
		message := map[string]interface{}{"type": "peer-joined", "peer": peer.GetInfo()}
		s.Send(otherPeer, message)
	}
	otherPeers := []Info{}
	for _, otherPeer := range s.Rooms[peer.Ip] {
		otherPeers = append(otherPeers, otherPeer.GetInfo())

	}
	message := map[string]interface{}{"type": "peers", "peers": otherPeers}
	s.Send(peer, message)
	s.Rooms[peer.Ip][peer.Id] = peer
}
func (s *SnapdropServer) LeaveRoom(peer *Peer) {
	if s.Rooms[peer.Ip] != nil && s.Rooms[peer.Ip][peer.Id] != nil {
		s.CancelKeepAlive(peer)
		delete(s.Rooms[peer.Ip], peer.Id)
		peer.Socket.Close()
		if len(s.Rooms[peer.Ip]) == 0 {
			delete(s.Rooms, peer.Ip)
		} else {
			for _, otherPeer := range s.Rooms[peer.Ip] {
				message := map[string]interface{}{"type": "peer-left", "peerId": peer.Id}
				s.Send(otherPeer, message)
			}
		}
	}
}
func (s *SnapdropServer) Timer(peer *Peer) {
	const timeout = time.Second * 30
	peer.Timer = true
	for {
		if !peer.Timer {
			break
		}
		s.KeepAlive(peer)
		time.Sleep(timeout)
	}
}
func (s *SnapdropServer) KeepAlive(peer *Peer) {
	const timeout = time.Second * 30
	if time.Since(peer.LastBeat) > 2*timeout {
		s.LeaveRoom(peer)
		return
	}
	message := map[string]interface{}{"type": "ping"}
	s.Send(peer, message)
}
func (s *SnapdropServer) CancelKeepAlive(peer *Peer) {
	peer.Timer = false
}
