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

func (h *controlHub) broadcastControl(state sim.Snapshot) {
	payload, err := proto.Marshal(stateMessage(state))
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
		h.sendState(conn, simulation.Snapshot())

		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				log.Printf("control stream read error: %v", err)
				return
			}

			var message pb.ControlMessage
			if err := proto.Unmarshal(data, &message); err != nil {
				log.Printf("unable to decode control message: %v", err)
				h.sendError(conn, "invalid control payload")
				continue
			}

			switch m := message.Control.(type) {
			case *pb.ControlMessage_Update:
				hospital := m.Update.GetHospital()
				settings := sim.ControlSettings{
					TransmissionModifier: m.Update.GetTransmissionRate(),
					LockdownEnabled:      m.Update.GetLockdownEnabled(),
				}
				if hospital != nil {
					settings.HospitalCapacity = int(hospital.GetCapacity())
					settings.DeathRateOverloadMultiplier = hospital.GetDeathRateOverloadMultiplier()
				}

				state := simulation.ApplyControlSettings(settings)
				h.sendAck(conn, state)
				h.broadcastControl(state)
			default:
				h.sendError(conn, "unsupported control message type")
			}
		}
	}
}

func (h *controlHub) sendState(conn *websocket.Conn, state sim.Snapshot) {
	if err := h.writeMessage(conn, stateMessage(state)); err != nil {
		log.Printf("failed to send control state: %v", err)
	}
}

func (h *controlHub) sendAck(conn *websocket.Conn, state sim.Snapshot) {
	ack := &pb.ControlMessage{
		Control: &pb.ControlMessage_Ack{
			Ack: &pb.ControlAck{Message: "applied control update", State: stateMessage(state).GetState()},
		},
	}
	if err := h.writeMessage(conn, ack); err != nil {
		log.Printf("failed to send control ack: %v", err)
	}
}

func (h *controlHub) sendError(conn *websocket.Conn, message string) {
	errMsg := &pb.ControlMessage{Control: &pb.ControlMessage_Error{Error: &pb.ControlError{Message: message}}}
	if err := h.writeMessage(conn, errMsg); err != nil {
		log.Printf("failed to send control error: %v", err)
	}
}

func (h *controlHub) writeMessage(conn *websocket.Conn, message *pb.ControlMessage) error {
	payload, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, payload)
}

func stateMessage(state sim.Snapshot) *pb.ControlMessage {
	return &pb.ControlMessage{Control: &pb.ControlMessage_State{State: snapshotToProto(state)}}
}

func snapshotToProto(state sim.Snapshot) *pb.ControlState {
	return &pb.ControlState{
		Settings: &pb.ControlUpdate{
			TransmissionRate: state.TransmissionModifier,
			LockdownEnabled:  state.LockdownEnabled,
			Hospital: &pb.HospitalParameters{
				Capacity:                    int32(state.HospitalCapacity),
				DeathRateOverloadMultiplier: state.DeathRateOverloadMultiplier,
			},
		},
		CurrentInfected:           int32(state.CurrentInfected),
		EffectiveDeathProbability: state.EffectiveDeathProbability,
		Overloaded:                state.Overloaded,
		InfectionProbability:      state.InfectionProbability,
		SpeedModifier:             state.SpeedModifier,
		CapacityUtilization:       state.CapacityUtilization,
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

	go simulation.Run(ctx, time.Second, func(state sim.Snapshot) {
		// Broadcast computed modifier so clients stay in sync.
		hub.broadcastControl(state)
		log.Printf(
			"tick probability=%.3f modifier=%.2f infected=%d overloaded=%t death_prob=%.3f",
			state.InfectionProbability,
			state.TransmissionModifier,
			state.CurrentInfected,
			state.Overloaded,
			state.EffectiveDeathProbability,
		)
	})

	http.Handle("/proto/", http.StripPrefix("/proto/", http.FileServer(http.Dir("proto"))))
	http.Handle("/ws/control", hub.handler(simulation))
	http.Handle("/", http.FileServer(http.Dir("web")))

	log.Printf("serving UI on http://localhost%v", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
