const slider = document.getElementById('transmissionSlider');
const valueEl = document.getElementById('transmissionValue');

let ControlUpdate;
let socket;

function updateDisplay(value) {
  const numeric = Number(value) || 0;
  valueEl.textContent = `${numeric.toFixed(2)}x`;
}

function sendUpdate(value) {
  if (!ControlUpdate || !socket || socket.readyState !== WebSocket.OPEN) {
    return;
  }
  const message = ControlUpdate.create({ transmission_modifier: Number(value) });
  const payload = ControlUpdate.encode(message).finish();
  socket.send(payload);
}

function attachSliderHandlers() {
  ['input', 'change'].forEach((evt) => {
    slider.addEventListener(evt, (event) => {
      const value = event.target.value;
      updateDisplay(value);
      sendUpdate(value);
    });
  });
}

function connectWebSocket() {
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
  socket = new WebSocket(`${protocol}://${window.location.host}/ws/control`);
  socket.binaryType = 'arraybuffer';

  socket.addEventListener('message', (event) => {
    if (!ControlUpdate) {
      return;
    }
    const data = new Uint8Array(event.data);
    const update = ControlUpdate.decode(data);
    const modifier = update.transmission_modifier ?? 1;
    slider.value = modifier;
    updateDisplay(modifier);
  });

  socket.addEventListener('close', () => {
    setTimeout(connectWebSocket, 1500);
  });
}

async function init() {
  const root = await protobuf.load('/proto/control.proto');
  ControlUpdate = root.lookupType('pandemica.ControlUpdate');
  attachSliderHandlers();
  connectWebSocket();
  updateDisplay(slider.value);
}

init().catch((err) => console.error('Failed to bootstrap control UI', err));
