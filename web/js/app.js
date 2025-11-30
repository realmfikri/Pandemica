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

let ControlUpdate;
let socket;
let lockdownEnabled = false;
let hospitalCapacity = Number(capacityInput.value) || 0;
let overloadMultiplier = Number(overloadInput.value) || 1;
let currentInfected = 0;
let overloaded = false;

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
  if (!ControlUpdate || !socket || socket.readyState !== WebSocket.OPEN) {
    return;
  }
  const message = ControlUpdate.create({
    transmission_modifier: Number(value),
    lockdown_enabled: lockdownEnabled,
    hospital_capacity: Number(capacityInput.value) || 0,
    death_rate_overload_multiplier: Number(overloadInput.value) || 1,
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

  [capacityInput, overloadInput].forEach((input) => {
    input.addEventListener('change', () => {
      hospitalCapacity = Number(capacityInput.value) || 0;
      overloadMultiplier = Number(overloadInput.value) || 1;
      sendUpdate(slider.value);
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
    const lockdown = update.lockdown_enabled ?? false;
    const capacity = update.hospital_capacity ?? hospitalCapacity;
    const overload = update.death_rate_overload_multiplier ?? overloadMultiplier;
    const infected = update.current_infected ?? currentInfected;
    const deathProbability = update.effective_death_probability ?? 0;
    const overloadedState = update.overloaded ?? false;
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
  updateHealthDisplay({
    infected: currentInfected,
    capacity: hospitalCapacity,
    deathProbability: 0,
    overloadedState: overloaded,
  });
}

init().catch((err) => console.error('Failed to bootstrap control UI', err));
