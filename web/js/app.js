const slider = document.getElementById('transmissionSlider');
const valueEl = document.getElementById('transmissionValue');
const lockdownToggle = document.getElementById('lockdownToggle');
const lockdownStatus = document.getElementById('lockdownStatus');
const rootStyle = document.documentElement.style;
const capacityInput = document.getElementById('hospitalCapacity');
const overloadInput = document.getElementById('overloadMultiplier');
const infectedEl = document.getElementById('infectedCount');
const deathEl = document.getElementById('deathProbability');
const capacityBanner = document.getElementById('capacityBanner');

let ControlMessage;
let ControlUpdate;
let HospitalParameters;
let socket;
let lockdownEnabled = false;
let hospitalCapacity = Number(capacityInput.value) || 0;
let overloadMultiplier = Number(overloadInput.value) || 1;
let currentInfected = 0;
let overloaded = false;
const networkStatus = document.getElementById('networkStatus');

function setNetworkStatus(message, isError = false) {
  if (!networkStatus) return;
  networkStatus.textContent = message;
  networkStatus.classList.toggle('error', isError);
}

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

function updateHealthDisplay({ infected = currentInfected, capacity = hospitalCapacity, deathProbability = 0, overloadedState = overloaded }) {
  currentInfected = infected;
  hospitalCapacity = capacity;
  overloaded = overloadedState;

  infectedEl.textContent = infected.toLocaleString();
  deathEl.textContent = `${(deathProbability * 100).toFixed(2)}%`;
  capacityInput.value = capacity;

  const safe = capacity <= 0 || infected <= capacity;
  capacityBanner.classList.toggle('overloaded', !safe);
  capacityBanner.textContent = !safe
    ? `Hospital overload: ${infected.toLocaleString()} infected exceeds capacity of ${capacity.toLocaleString()}. Death risk is amplified.`
    : `Capacity holding: ${infected.toLocaleString()} infected of ${capacity.toLocaleString()} slots. Baseline death rate applies.`;
}

function sendUpdate(value) {
  if (!ControlUpdate || !ControlMessage || !HospitalParameters || !socket || socket.readyState !== WebSocket.OPEN) {
    return;
  }
  const hospital = HospitalParameters.create({
    capacity: Number(capacityInput.value) || 0,
    death_rate_overload_multiplier: Number(overloadInput.value) || 1,
  });
  const update = ControlUpdate.create({
    transmission_rate: Number(value),
    lockdown_enabled: lockdownEnabled,
    hospital,
  });
  const payload = ControlMessage.encode(
    ControlMessage.create({
      update,
    })
  ).finish();
  socket.send(payload);
  setNetworkStatus('Sending update...', false);
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

  [capacityInput, overloadInput].forEach((input) => {
    input.addEventListener('change', () => {
      hospitalCapacity = Number(capacityInput.value) || 0;
      overloadMultiplier = Number(overloadInput.value) || 1;
      sendUpdate(slider.value);
    });
  });
}

function applyState(state) {
  if (!state) return;
  const settings = state.settings || {};
  const hospital = settings.hospital || {};
  const modifier = settings.transmission_rate ?? slider.value;
  const lockdown = settings.lockdown_enabled ?? false;
  const capacity = hospital.capacity ?? hospitalCapacity;
  const overload = hospital.death_rate_overload_multiplier ?? overloadMultiplier;
  const infected = state.current_infected ?? currentInfected;
  const deathProbability = state.effective_death_probability ?? 0;
  const overloadedState = state.overloaded ?? false;

  slider.value = modifier;
  updateDisplay(modifier);
  applyLockdownUI(lockdown);
  overloadMultiplier = Number(overload) || 1;
  overloadInput.value = overloadMultiplier;
  updateHealthDisplay({
    infected,
    capacity,
    deathProbability,
    overloadedState,
  });
}

function connectWebSocket() {
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
  socket = new WebSocket(`${protocol}://${window.location.host}/ws/control`);
  socket.binaryType = 'arraybuffer';

  socket.addEventListener('message', (event) => {
    if (!ControlMessage) {
      return;
    }
    const data = new Uint8Array(event.data);
    const message = ControlMessage.decode(data);
    if (message.state) {
      applyState(message.state);
      setNetworkStatus('Live state synchronized.', false);
    }
    if (message.ack) {
      setNetworkStatus(message.ack.message || 'Update acknowledged.', false);
      if (message.ack.state) {
        applyState(message.ack.state);
      }
    }
    if (message.error) {
      setNetworkStatus(message.error.message || 'Update rejected.', true);
    }
  });

  socket.addEventListener('close', () => {
    setTimeout(connectWebSocket, 1500);
  });
}

async function init() {
  const root = await protobuf.load('/proto/control.proto');
  ControlMessage = root.lookupType('pandemica.ControlMessage');
  ControlUpdate = root.lookupType('pandemica.ControlUpdate');
  HospitalParameters = root.lookupType('pandemica.HospitalParameters');
  attachSliderHandlers();
  connectWebSocket();
  updateDisplay(slider.value);
  applyLockdownUI(lockdownEnabled);
  updateHealthDisplay({
    infected: currentInfected,
    capacity: hospitalCapacity,
    deathProbability: 0,
    overloadedState: overloaded,
  });
}

init().catch((err) => console.error('Failed to bootstrap control UI', err));
