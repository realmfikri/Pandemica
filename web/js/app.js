const slider = document.getElementById('transmissionSlider');
const valueEl = document.getElementById('transmissionValue');
const lockdownToggle = document.getElementById('lockdownToggle');
const lockdownStatus = document.getElementById('lockdownStatus');
const rootStyle = document.documentElement.style;

let ControlUpdate;
let socket;
let lockdownEnabled = false;

function updateDisplay(value) {
  const numeric = Number(value) || 0;
  valueEl.textContent = `${numeric.toFixed(2)}x`;
}

function applyLockdownUI(enabled) {
  lockdownEnabled = enabled;
  lockdownToggle.checked = enabled;
  document.body.classList.toggle('lockdown', enabled);
  lockdownStatus.textContent = enabled
    ? 'Lockdown active — agents slowed.'
    : 'Lockdown inactive — agents moving normally.';

  const speedModifier = enabled ? 0.1 : 1;
  const durationSeconds = 8 / Math.max(speedModifier, 0.05);
  rootStyle.setProperty('--speed-modifier', speedModifier);
  rootStyle.setProperty('--agent-duration', `${durationSeconds.toFixed(2)}s`);
}

function sendUpdate(value) {
  if (!ControlUpdate || !socket || socket.readyState !== WebSocket.OPEN) {
    return;
  }
  const message = ControlUpdate.create({
    transmission_modifier: Number(value),
    lockdown_enabled: lockdownEnabled,
  });
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

  lockdownToggle.addEventListener('change', (event) => {
    applyLockdownUI(event.target.checked);
    sendUpdate(slider.value);
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
    const lockdown = update.lockdown_enabled ?? false;
    slider.value = modifier;
    updateDisplay(modifier);
    applyLockdownUI(lockdown);
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
  applyLockdownUI(lockdownEnabled);
}

init().catch((err) => console.error('Failed to bootstrap control UI', err));
