package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	sim "pandemica/internal/sim"
	pb "pandemica/proto"
)

type controlHub struct {
	mu       sync.Mutex
	clients  map[*websocket.Conn]struct{}
	upgrader websocket.Upgrader
}

func newControlHub() *controlHub {
	return &controlHub{
		clients: make(map[*websocket.Conn]struct{}),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (h *controlHub) add(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[conn] = struct{}{}
}

func (h *controlHub) remove(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, conn)
	conn.Close()
}

func (h *controlHub) broadcastControl(modifier float64) {
	payload, err := proto.Marshal(&pb.ControlUpdate{TransmissionModifier: modifier})
	if err != nil {
		log.Printf("failed to marshal control update: %v", err)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for conn := range h.clients {
		if err := conn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
			log.Printf("failed to write to client: %v", err)
			conn.Close()
			delete(h.clients, conn)
		}
	}
}

func (h *controlHub) handler(simulation *sim.Simulation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := h.upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websocket upgrade failed: %v", err)
			return
		}
		h.add(conn)
		defer h.remove(conn)

		// Send the current control state immediately.
		h.broadcastControl(simulation.CurrentTransmissionModifier())

		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				log.Printf("control stream read error: %v", err)
				return
			}

			var update pb.ControlUpdate
			if err := proto.Unmarshal(data, &update); err != nil {
				log.Printf("unable to decode control update: %v", err)
				continue
			}

			simulation.UpdateTransmissionModifier(update.GetTransmissionModifier())
			h.broadcastControl(simulation.CurrentTransmissionModifier())
		}
	}
}

func main() {
	addr := flag.String("addr", ":8080", "server listen address")
	base := flag.Float64("base", 0.25, "base transmission probability")
	flag.Parse()

	simulation := sim.New(*base)
	hub := newControlHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go simulation.Run(ctx, time.Second, func(probability, modifier float64) {
		// Broadcast computed modifier so clients stay in sync.
		hub.broadcastControl(modifier)
		log.Printf("tick probability=%.3f modifier=%.2f", probability, modifier)
	})

	http.Handle("/proto/", http.StripPrefix("/proto/", http.FileServer(http.Dir("proto"))))
	http.Handle("/ws/control", hub.handler(simulation))
	http.Handle("/", http.FileServer(http.Dir("web")))

	log.Printf("serving UI on http://localhost%v", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
