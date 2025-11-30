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
const speedBadge = document.getElementById('speedBadge');
const lockdownBadge = document.getElementById('lockdownBadge');
const capacityBar = document.getElementById('capacityBar');
const capacityValue = document.getElementById('capacityValue');
const infectionProbabilityEl = document.getElementById('infectionProbabilityValue');
const chartCanvas = document.getElementById('curveChart');

let infectionChart;
let timeIndex = 0;
const infectionData = [];
const deathData = [];
const interventionMarkers = [];
let lastPhaseLabel = 'Baseline (1.00x)';

let ControlMessage;
let ControlUpdate;
let HospitalParameters;
let socket;
let lockdownEnabled = false;
let hospitalCapacity = Number(capacityInput.value) || 0;
let overloadMultiplier = Number(overloadInput.value) || 1;
let currentInfected = 0;
let overloaded = false;
let speedModifier = 1;
const networkStatus = document.getElementById('networkStatus');

function setNetworkStatus(message, isError = false) {
  if (!networkStatus) return;
  networkStatus.textContent = message;
  networkStatus.classList.toggle('error', isError);
}

function phaseFromState({ lockdown, modifier }) {
  const speedLabel = lockdown ? 'Lockdown' : 'Open';
  return `${speedLabel} (${Number(modifier || 1).toFixed(2)}x)`;
}

function addInterventionMarker(label) {
  interventionMarkers.push({ x: timeIndex, label });
  if (interventionMarkers.length > 20) {
    interventionMarkers.shift();
  }
}

const interventionLinesPlugin = {
  id: 'interventionLines',
  afterDatasetsDraw(chart) {
    const markers = interventionMarkers;
    if (!markers.length) return;

    const {
      ctx,
      chartArea: { top, bottom },
      scales: { x },
    } = chart;
    ctx.save();
    ctx.setLineDash([6, 4]);
    ctx.strokeStyle = '#f97316';
    ctx.fillStyle = '#e2e8f0';
    ctx.font = '12px sans-serif';

    markers.forEach((marker) => {
      const xPos = x.getPixelForValue(marker.x);
      ctx.beginPath();
      ctx.moveTo(xPos, top);
      ctx.lineTo(xPos, bottom);
      ctx.stroke();
      ctx.fillText(marker.label, xPos + 4, top + 12);
    });

    ctx.restore();
  },
};

function createChart() {
  if (!chartCanvas || !window.Chart) return;
  infectionChart = new Chart(chartCanvas.getContext('2d'), {
    type: 'line',
    data: {
      datasets: [
        {
          label: 'Current infected',
          data: infectionData,
          parsing: false,
          borderColor: '#38bdf8',
          backgroundColor: 'rgba(56, 189, 248, 0.1)',
          borderWidth: 2,
          fill: true,
          segment: {
            borderColor: (ctx) =>
              ctx.p1?.raw?.phase?.includes('Lockdown') ? '#f97316' : '#38bdf8',
            backgroundColor: (ctx) =>
              ctx.p1?.raw?.phase?.includes('Lockdown')
                ? 'rgba(249, 115, 22, 0.15)'
                : 'rgba(56, 189, 248, 0.1)',
          },
        },
        {
          label: 'Death probability (%)',
          data: deathData,
          parsing: false,
          borderColor: '#e11d48',
          backgroundColor: 'rgba(225, 29, 72, 0.15)',
          borderWidth: 2,
          fill: true,
          yAxisID: 'y1',
          segment: {
            borderColor: (ctx) =>
              ctx.p1?.raw?.phase?.includes('Lockdown') ? '#fb7185' : '#e11d48',
            backgroundColor: (ctx) =>
              ctx.p1?.raw?.phase?.includes('Lockdown')
                ? 'rgba(251, 113, 133, 0.15)'
                : 'rgba(225, 29, 72, 0.15)',
          },
        },
      ],
    },
    options: {
      animation: false,
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: 'nearest', intersect: false },
      scales: {
        x: {
          type: 'linear',
          title: { display: true, text: 'Simulation ticks' },
        },
        y: {
          title: { display: true, text: 'Infected agents' },
          ticks: { precision: 0 },
        },
        y1: {
          position: 'right',
          title: { display: true, text: 'Death probability (%)' },
          min: 0,
          max: 100,
          grid: { drawOnChartArea: false },
        },
      },
      plugins: {
        legend: { position: 'bottom' },
        tooltip: {
          callbacks: {
            title: (context) => `Tick ${context[0].parsed.x}`,
            afterBody: (items) => {
              const phase = items[0]?.raw?.phase;
              return phase ? `Phase: ${phase}` : '';
            },
          },
        },
      },
    },
    plugins: [interventionLinesPlugin],
  });
}

function updateDisplay(value) {
  const numeric = Number(value) || 0;
  valueEl.textContent = `${numeric.toFixed(2)}x`;
}

function updateIndicators({
  infectionProbability = 0,
  speed = speedModifier,
  capacityUtilization = 0,
  overloadedState = overloaded,
  modifier = slider.value,
  lockdown = lockdownEnabled,
} = {}) {
  if (infectionProbabilityEl) {
    infectionProbabilityEl.textContent = `${(infectionProbability * 100).toFixed(1)}% chance per contact`;
  }
  if (speedBadge) {
    speedBadge.textContent = `${Number(speed).toFixed(2)}x movement speed`;
  }
  if (lockdownBadge) {
    lockdownBadge.textContent = lockdown ? 'Lockdown active' : 'Lockdown open';
    lockdownBadge.classList.toggle('active', lockdown);
  }
  const utilizationPct = capacityUtilization * 100;
  if (capacityBar) {
    const bounded = Math.min(Math.max(utilizationPct, 0), 200);
    capacityBar.style.width = `${bounded}%`;
    capacityBar.classList.toggle('over', overloadedState || utilizationPct > 100);
  }
  if (capacityValue) {
    const descriptor = overloadedState ? 'Over capacity' : 'Within capacity';
    capacityValue.textContent = `${utilizationPct.toFixed(0)}% — ${descriptor}`;
  }

  const newPhase = phaseFromState({ lockdown, modifier });
  if (newPhase !== lastPhaseLabel) {
    addInterventionMarker(newPhase);
    lastPhaseLabel = newPhase;
  }
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

function updateHealthDisplay({
  infected = currentInfected,
  capacity = hospitalCapacity,
  deathProbability = 0,
  overloadedState = overloaded,
}) {
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

function updateCharts({ infected, deathProbability, infectionProbability, modifier, lockdown }) {
  if (!infectionChart) return;
  timeIndex += 1;
  const phase = phaseFromState({ lockdown, modifier });
  infectionData.push({ x: timeIndex, y: infected, phase });
  deathData.push({ x: timeIndex, y: (deathProbability || 0) * 100, phase });

  const maxPoints = 240;
  if (infectionData.length > maxPoints) infectionData.shift();
  if (deathData.length > maxPoints) deathData.shift();

  infectionChart.update('none');
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
  const infectionProbability = state.infection_probability ?? 0;
  speedModifier = state.speed_modifier ?? speedModifier;
  const capacityUtilization = state.capacity_utilization ?? 0;

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
  updateIndicators({
    infectionProbability,
    speed: speedModifier,
    capacityUtilization,
    overloadedState,
    modifier,
    lockdown,
  });
  updateCharts({
    infected,
    deathProbability,
    infectionProbability,
    modifier,
    lockdown,
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
  createChart();
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
  updateIndicators({
    infectionProbability: 0,
    speed: speedModifier,
    capacityUtilization: 0,
    overloadedState: overloaded,
    modifier: slider.value,
    lockdown: lockdownEnabled,
  });
}

init().catch((err) => console.error('Failed to bootstrap control UI', err));
