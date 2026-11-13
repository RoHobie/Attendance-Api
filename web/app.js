const $ = (id) => document.getElementById(id);
let code = sessionStorage.getItem('code') || '';
let me = null;
let students = [];
let saved = {};   // student id -> { id, status } already stored for the chosen class, date and session
let marks = {};   // student id -> 'PRESENT' | 'ABSENT' currently selected on screen

async function call(method, path, body) {
  const res = await fetch(path, {
    method,
    headers: { 'Authorization': 'Bearer ' + code, 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined,
  });
  let data = {};
  try { data = await res.json(); } catch (e) {}
  return { ok: res.ok, status: res.status, data };
}

function say(text, kind) {
  const m = $('message');
  m.textContent = text || '';
  m.className = kind || '';
  m.hidden = !text;
}

function today() {
  const d = new Date();
  d.setMinutes(d.getMinutes() - d.getTimezoneOffset());
  return d.toISOString().slice(0, 10);
}

async function start() {
  const r = await call('GET', '/api/me');
  if (!r.ok) {
    sessionStorage.removeItem('code');
    $('login').hidden = false;
    $('app').hidden = true;
    return false;
  }
  me = r.data.data;
  $('who').textContent = me.name;
  $('login').hidden = true;
  $('app').hidden = false;
  const canEdit = me.role === 'TEACHER';
  $('actions').hidden = !canEdit;

  const c = await call('GET', '/api/classes');
  const classes = c.data.data || [];
  $('class').innerHTML = classes.map(k => `<option value="${k.id}">${k.name}</option>`).join('');
  if (!$('date').value) $('date').value = today();
  if (!classes.length) { say('No classes are assigned to you yet.'); $('rows').innerHTML = ''; return true; }
  await load();
  return true;
}

async function load() {
  say('');
  const cls = $('class').value, date = $('date').value, session = $('session').value;
  if (!cls || !date) return;
  const [s, a] = await Promise.all([
    call('GET', `/api/classes/${cls}/students`),
    call('GET', `/api/classes/${cls}/attendance?date=${date}`),
  ]);
  if (!s.ok || !a.ok) { say('Could not load this class. Please try again.', 'error'); return; }
  students = s.data.data;
  saved = {}; marks = {};
  for (const r of a.data.data) {
    if (r.session === session) { saved[r.student_id] = { id: r.id, status: r.status }; marks[r.student_id] = r.status; }
  }
  render();
}

function render() {
  const canEdit = me.role === 'TEACHER';
  $('rows').innerHTML = students.map((st, i) => `
    <tr>
      <td>${i + 1}</td>
      <td>${st.name}</td>
      <td><input type="radio" name="s${st.id}" data-id="${st.id}" value="PRESENT" ${marks[st.id] === 'PRESENT' ? 'checked' : ''} ${canEdit ? '' : 'disabled'}></td>
      <td><input type="radio" name="s${st.id}" data-id="${st.id}" value="ABSENT" ${marks[st.id] === 'ABSENT' ? 'checked' : ''} ${canEdit ? '' : 'disabled'}></td>
    </tr>`).join('');
  summary();
}

function summary() {
  const v = Object.values(marks);
  const p = v.filter(x => x === 'PRESENT').length, ab = v.filter(x => x === 'ABSENT').length;
  $('summary').textContent = `${students.length} students · ${p} present · ${ab} absent · ${students.length - p - ab} not marked`;
}

$('rows').addEventListener('change', (e) => {
  if (e.target.dataset.id) { marks[e.target.dataset.id] = e.target.value; summary(); }
});

$('all-present').addEventListener('click', () => {
  students.forEach(s => marks[s.id] = 'PRESENT');
  render();
});

$('save').addEventListener('click', async () => {
  const cls = $('class').value, date = $('date').value, session = $('session').value;
  let done = 0, failed = 0;
  $('save').disabled = true;
  for (const st of students) {
    const want = marks[st.id];
    if (!want) continue;
    const have = saved[st.id];
    let r;
    if (!have) r = await call('POST', `/api/classes/${cls}/attendance`, { student_id: st.id, date, session, status: want });
    else if (have.status !== want) r = await call('PUT', `/api/classes/${cls}/attendance/${have.id}`, { status: want });
    else continue;
    if (r.ok) done++; else failed++;
  }
  $('save').disabled = false;
  await load();
  if (failed) say(`${done} saved, ${failed} could not be saved.`, 'error');
  else say(done ? `Saved ${done} change${done > 1 ? 's' : ''}.` : 'Nothing to save.', done ? 'ok' : '');
});

for (const id of ['class', 'date', 'session']) $(id).addEventListener('change', load);

$('login-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  code = $('code').value.trim();
  sessionStorage.setItem('code', code);
  $('login-error').hidden = true;
  if (!(await start())) { $('login-error').textContent = 'That access code was not recognised.'; $('login-error').hidden = false; }
});

$('logout').addEventListener('click', () => {
  sessionStorage.removeItem('code');
  code = ''; $('code').value = '';
  $('app').hidden = true; $('login').hidden = false;
});

if (code) start();
