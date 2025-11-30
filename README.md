# Pandemica

A lightweight sandbox that demonstrates a controllable virus transmission modifier. The UI exposes a slider that updates the simulation in real time via a protobuf-backed WebSocket channel.

## Running the demo

```bash
go run ./cmd/server
```

Open http://localhost:8080 in your browser to reach the control panel.

## Transmission modifier control

- The slider ranges from **0.00** to **1.00** and scales the base infection probability used by the Go simulation loop.
- Moving the slider sends a `ControlUpdate` protobuf message to the server; the server applies it immediately and echoes the current value back to all connected clients so everyone stays synchronized.
- Leaving the slider at **1.00** preserves the default transmission behavior, while lowering it suppresses the chance that one agent infects another during a tick.

## Hospital capacity & overload

- The control panel exposes **Hospital capacity** (number of simultaneous infections that can be treated) and an **Overload death multiplier** (how sharply deaths rise when capacity is exceeded).
- Every tick the backend tracks the current infected count. If `infected > capacity`, the per-tick death probability is multiplied by the overload factor; otherwise the baseline fatality rate is used.
- The dashboard banner displays whether the system is holding or overloaded so you can see when deaths are accelerating.
- For calmer runs, raise capacity or lower the overload multiplier. To stress the system, drop capacity or raise the multiplier and watch the banner turn red as deaths spike.

The in-app help overlay mirrors this information so players can see how their adjustments affect the underlying infection probability.
