'use strict';

// Halaman kelola nomor WhatsApp Gateway. Semua data lewat /api/v1 dengan kunci master;
// tanpa kunci yang benar halaman ini tidak bisa melihat atau mengubah apa pun.
(() => {
  const API = '/api/v1';
  const KEY_STORE = 'wa-gateway-master-key';
  const LIST_INTERVAL = 5000;
  const CONNECT_INTERVAL = 2000;

  const $ = (id) => document.getElementById(id);
  const state = { key: '', sessions: [], lastJson: '', listTimer: null, loading: false };

  // ---------- Penyimpanan kunci: per tab, atau permanen bila "Ingat" dicentang ----------
  function store(kind) {
    try { return window[kind] || null; } catch { return null; }
  }
  function loadKey() {
    for (const kind of ['sessionStorage', 'localStorage']) {
      try {
        const v = store(kind)?.getItem(KEY_STORE);
        if (v) return v;
      } catch { /* penyimpanan diblokir */ }
    }
    return '';
  }
  function clearKey() {
    for (const kind of ['sessionStorage', 'localStorage']) {
      try { store(kind)?.removeItem(KEY_STORE); } catch { /* abaikan */ }
    }
  }
  function saveKey(key, remember) {
    clearKey();
    try { store(remember ? 'localStorage' : 'sessionStorage')?.setItem(KEY_STORE, key); } catch { /* abaikan */ }
  }

  // ---------- API ----------
  class ApiError extends Error {
    constructor(status, message, code, details) {
      super(message);
      this.status = status;
      this.code = code;
      this.details = details;
    }
  }

  async function api(path, { method = 'GET', body } = {}) {
    let res;
    try {
      res = await fetch(API + path, {
        method,
        cache: 'no-store',
        headers: { 'x-api-key': state.key, ...(body ? { 'Content-Type': 'application/json' } : {}) },
        body: body ? JSON.stringify(body) : undefined,
      });
    } catch {
      throw new ApiError(0, 'Tidak bisa menghubungi server. Periksa koneksi internet.');
    }
    let json = null;
    try { json = await res.json(); } catch { /* bukan JSON */ }
    if (!res.ok || (json && json.success === false)) {
      const e = json?.error || {};
      throw new ApiError(res.status, e.message || `Permintaan gagal (${res.status}).`, e.code, e.details);
    }
    return json?.data ?? json;
  }

  const sid = (id) => `/sessions/${encodeURIComponent(id)}`;

  // ---------- Bantuan tampilan ----------
  function h(tag, props = {}, ...children) {
    const el = document.createElement(tag);
    for (const [k, v] of Object.entries(props)) {
      if (v == null || v === false) continue;
      if (k === 'class') el.className = v;
      else if (k === 'text') el.textContent = v;
      else if (k.startsWith('on')) el.addEventListener(k.slice(2), v);
      else el.setAttribute(k, v === true ? '' : v);
    }
    for (const c of children.flat()) {
      if (c == null || c === false) continue;
      el.append(c instanceof Node ? c : document.createTextNode(String(c)));
    }
    return el;
  }

  const ICON_PHONE = '<svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true"><path fill="currentColor" d="M7 2h10a2 2 0 0 1 2 2v16a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2Zm0 2v16h10V4H7Zm4 13h2v2h-2z"/></svg>';

  function formatPhone(n) {
    const d = String(n || '').replace(/\D/g, '');
    if (!d) return '';
    if (d.startsWith('62')) return '+62 ' + d.slice(2).replace(/^(\d{3})(\d{4})(\d+)$/, '$1-$2-$3');
    return '+' + d;
  }

  function normalizePhone(input) {
    let d = String(input || '').replace(/\D/g, '');
    if (d.startsWith('0')) d = '62' + d.slice(1);
    else if (d.startsWith('8')) d = '62' + d;
    return d;
  }

  const rtf = new Intl.RelativeTimeFormat('id', { numeric: 'auto' });
  function ago(iso) {
    const t = Date.parse(iso);
    if (!t) return '';
    const s = Math.round((t - Date.now()) / 1000);
    const abs = Math.abs(s);
    if (abs < 45) return 'baru saja';
    if (abs < 3600) return rtf.format(Math.round(s / 60), 'minute');
    if (abs < 86400) return rtf.format(Math.round(s / 3600), 'hour');
    return rtf.format(Math.round(s / 86400), 'day');
  }

  // Gabungan status di database + keadaan socket di memori server.
  function view(s) {
    if (s.runtime === 'socket' && s.status === 'CONNECTED') return { key: 'connected', label: 'Terhubung', tone: 'ok' };
    if (s.runtime === 'socket' && (s.status === 'QR_PENDING' || s.status === 'STARTING')) return { key: 'qr', label: 'Menunggu pindai', tone: 'warn' };
    if (s.runtime === 'socket' || s.runtime === 'pending') return { key: 'pending', label: 'Menyambungkan…', tone: 'warn' };
    if (s.status === 'LOGGED_OUT') return { key: 'stopped', label: 'Dikeluarkan dari HP', tone: '' };
    return { key: 'stopped', label: 'Terputus', tone: 'bad' };
  }

  function errorText(err, fallback) {
    if (err.code === 'VALIDATION_ERROR' && Array.isArray(err.details) && err.details[0]) {
      const field = err.details[0].path?.[0];
      if (field === 'sessionId') return 'Nama sesi hanya boleh huruf, angka, - dan _ (3–64 karakter).';
      if (field === 'webhookUrl') return 'Alamat webhook tidak valid.';
      if (field === 'phoneNumber') return 'Nomor hanya angka dengan kode negara, mis. 6281234567890.';
    }
    if (err.code === 'ALREADY_EXISTS') return 'Nama sesi ini sudah dipakai. Pilih nama lain.';
    if (err.code === 'NOT_ACTIVE') return 'Sesi belum siap. Tunggu kode QR muncul, lalu coba lagi.';
    if (err.status === 0) return err.message;
    return fallback ? `${fallback} ${err.message}` : err.message;
  }

  let toastTimer = null;
  function toast(msg) {
    const t = $('toast');
    t.textContent = msg;
    t.hidden = false;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => { t.hidden = true; }, 3200);
  }

  function showAlert(el, msg) {
    el.textContent = msg || '';
    el.hidden = !msg;
  }

  // ---------- Masuk / keluar ----------
  function showLogin(msg) {
    stopListPolling();
    state.key = '';
    state.lastJson = '';
    $('view-main').hidden = true;
    $('btn-logout').hidden = true;
    $('view-login').hidden = false;
    showAlert($('login-error'), msg);
    $('login-key').value = '';
    $('login-key').focus();
  }

  function showMain() {
    $('view-login').hidden = true;
    $('view-main').hidden = false;
    $('btn-logout').hidden = false;
    startListPolling();
  }

  $('form-login').addEventListener('submit', async (e) => {
    e.preventDefault();
    const key = $('login-key').value.trim();
    if (!key) return;
    const btn = e.submitter || e.target.querySelector('button[type=submit]');
    btn.disabled = true;
    showAlert($('login-error'), '');
    state.key = key;
    try {
      const list = await api('/sessions');
      saveKey(key, $('login-remember').checked);
      renderList(list);
      showMain();
    } catch (err) {
      state.key = '';
      showAlert($('login-error'), err.status === 401 ? 'Kunci master salah.' : errorText(err));
    } finally {
      btn.disabled = false;
    }
  });

  $('btn-logout').addEventListener('click', () => {
    clearKey();
    showLogin('');
  });

  // ---------- Daftar nomor ----------
  function startListPolling() {
    stopListPolling();
    state.listTimer = setInterval(() => {
      if (document.visibilityState === 'visible') refresh();
    }, LIST_INTERVAL);
  }
  function stopListPolling() {
    clearInterval(state.listTimer);
    state.listTimer = null;
  }

  async function refresh() {
    if (!state.key || state.loading) return;
    state.loading = true;
    try {
      renderList(await api('/sessions'));
      showAlert($('list-error'), '');
    } catch (err) {
      if (err.status === 401) {
        clearKey();
        showLogin('Kunci master tidak berlaku lagi. Masuk ulang.');
      } else {
        showAlert($('list-error'), errorText(err, 'Gagal memuat daftar.'));
      }
    } finally {
      state.loading = false;
    }
  }

  function renderSkeleton() {
    $('list').replaceChildren(...[0, 1].map(() => h('div', { class: 'card skeleton', 'aria-hidden': 'true' })));
  }

  function renderList(list) {
    const sessions = Array.isArray(list) ? list.slice() : [];
    sessions.sort((a, b) => String(a.created_at).localeCompare(String(b.created_at)));
    const json = JSON.stringify(sessions);
    const summary = summarize(sessions);
    $('summary').textContent = summary;
    if (json === state.lastJson) return; // tidak berubah: jangan gambar ulang (fokus tombol tetap)
    state.lastJson = json;
    state.sessions = sessions;

    $('empty').hidden = sessions.length > 0;
    $('list').replaceChildren(...sessions.map(renderSession));
  }

  function summarize(sessions) {
    if (!sessions.length) return 'Belum ada nomor tersambung.';
    const n = { connected: 0, wait: 0, stopped: 0 };
    for (const s of sessions) {
      const k = view(s).key;
      if (k === 'connected') n.connected++;
      else if (k === 'stopped') n.stopped++;
      else n.wait++;
    }
    const parts = [`${n.connected} terhubung`];
    if (n.wait) parts.push(`${n.wait} menunggu`);
    if (n.stopped) parts.push(`${n.stopped} terputus`);
    return parts.join(' · ');
  }

  function renderSession(s) {
    const v = view(s);
    const avatar = h('div', { class: `avatar ${v.tone}` });
    avatar.innerHTML = ICON_PHONE;

    const phone = s.phone_number
      ? h('span', { class: 'phone', text: formatPhone(s.phone_number) })
      : h('span', { class: 'phone none', text: 'Nomor belum tertaut' });

    const meta = h('div', { class: 'meta' },
      h('span', {}, 'Sesi ', h('code', { text: s.id })),
      s.updated_at ? h('span', { text: `Diperbarui ${ago(s.updated_at)}` }) : null,
      s.webhook_url ? h('span', { title: s.webhook_url, text: `Webhook: ${s.webhook_url}` }) : null,
    );

    const actions = h('div', { class: 'actions' });
    if (v.key === 'qr') {
      actions.append(h('button', { class: 'btn primary small', type: 'button', onclick: () => openConnect(s.id) }, 'Tampilkan QR'));
    } else if (v.key === 'pending') {
      actions.append(h('button', { class: 'btn small', type: 'button', onclick: () => openConnect(s.id) }, 'Lihat proses'));
    } else if (v.key === 'stopped') {
      actions.append(h('button', { class: 'btn primary small', type: 'button', onclick: (e) => reconnect(s, e.currentTarget) }, 'Sambungkan ulang'));
    }
    actions.append(h('button', {
      class: 'btn small danger-soft',
      type: 'button',
      onclick: () => askRemove(s),
    }, v.key === 'connected' ? 'Putuskan' : 'Hapus'));

    return h('article', { class: 'card session' },
      avatar,
      h('div', { class: 'session-main' },
        h('div', { class: 'session-title' }, phone, h('span', { class: `badge ${v.tone}`, text: v.label })),
        meta,
      ),
      actions,
    );
  }

  async function reconnect(s, btn) {
    btn.disabled = true;
    try {
      await api(`${sid(s.id)}/restart`, { method: 'POST' });
      openConnect(s.id);
    } catch (err) {
      toast(errorText(err, 'Gagal menyambungkan ulang.'));
    } finally {
      btn.disabled = false;
    }
  }

  // ---------- Dialog: umum ----------
  function closeDialog(dlg) {
    if (dlg.id === 'dlg-connect' && !canCloseConnect()) return;
    dlg.close();
  }
  document.querySelectorAll('[data-close]').forEach((btn) => {
    btn.addEventListener('click', () => closeDialog(btn.closest('dialog')));
  });
  document.querySelectorAll('dialog').forEach((dlg) => {
    dlg.addEventListener('cancel', (e) => {
      if (dlg.id === 'dlg-connect' && !canCloseConnect()) e.preventDefault();
    });
  });

  // ---------- Dialog: tambah nomor ----------
  function openAdd() {
    $('form-add').reset();
    showAlert($('add-error'), '');
    $('dlg-add').showModal();
    $('add-id').focus();
  }
  $('btn-add').addEventListener('click', openAdd);
  $('btn-add-empty').addEventListener('click', openAdd);

  $('form-add').addEventListener('submit', async (e) => {
    e.preventDefault();
    const sessionId = $('add-id').value.trim();
    const webhookUrl = $('add-webhook').value.trim();
    const btn = $('add-submit');
    btn.disabled = true;
    showAlert($('add-error'), '');
    try {
      const data = await api('/sessions', {
        method: 'POST',
        body: webhookUrl ? { sessionId, webhookUrl } : { sessionId },
      });
      $('dlg-add').close();
      openConnect(sessionId, data.apiKey);
      refresh();
    } catch (err) {
      showAlert($('add-error'), errorText(err, 'Gagal menambah nomor.'));
    } finally {
      btn.disabled = false;
    }
  });

  // ---------- Dialog: sambungkan (QR / kode tautan) ----------
  const connect = { id: null, timer: null, busy: false, lastQr: null, keyPending: false, keyWarned: false, done: false };

  function canCloseConnect() {
    if (connect.keyPending && !connect.keyWarned) {
      connect.keyWarned = true;
      $('connect-key').scrollIntoView({ block: 'nearest' });
      toast('Salin dulu kunci API sesi ini — tidak bisa dilihat lagi. Tekan Tutup sekali lagi untuk tetap menutup.');
      return false;
    }
    return true;
  }

  function openConnect(id, apiKey) {
    Object.assign(connect, { id, busy: false, lastQr: null, keyPending: !!apiKey, keyWarned: false, done: false });
    $('connect-id').textContent = id;
    $('connect-key').hidden = !apiKey;
    $('connect-key-value').textContent = apiKey || '';
    $('connect-done').hidden = true;
    $('connect-flow').hidden = false;
    selectTab('qr');
    $('form-code').reset();
    $('code-result').hidden = true;
    showAlert($('code-error'), '');
    showQrWait('Menyiapkan kode QR…');
    setStatus('');
    $('dlg-connect').showModal();

    clearInterval(connect.timer);
    tick();
    connect.timer = setInterval(tick, CONNECT_INTERVAL);
  }

  $('dlg-connect').addEventListener('close', () => {
    clearInterval(connect.timer);
    connect.timer = null;
    connect.id = null;
    $('qr-img').removeAttribute('src');
    refresh();
  });

  $('connect-key-copy').addEventListener('click', async () => {
    const text = $('connect-key-value').textContent;
    try {
      await navigator.clipboard.writeText(text);
      connect.keyPending = false;
      toast('Kunci API disalin.');
    } catch {
      const range = document.createRange();
      range.selectNodeContents($('connect-key-value'));
      const sel = window.getSelection();
      sel.removeAllRanges();
      sel.addRange(range);
      connect.keyPending = false;
      toast('Kunci sudah ditandai — salin dengan Ctrl+C.');
    }
  });

  function setStatus(msg) { $('connect-status').textContent = msg || ''; }

  function showQrWait(text) {
    $('qr-img').hidden = true;
    $('qr-expired').hidden = true;
    $('qr-wait').hidden = false;
    $('qr-wait').querySelector('.spinner').hidden = false;
    $('qr-wait-text').textContent = text;
  }

  function showQrImage(src) {
    const img = $('qr-img');
    if (img.getAttribute('src') !== src) img.src = src;
    img.hidden = false;
    $('qr-wait').hidden = true;
    $('qr-expired').hidden = true;
  }

  function showQrExpired(s) {
    $('qr-img').hidden = true;
    $('qr-wait').hidden = true;
    $('qr-expired').hidden = false;
    const msg = $('qr-expired').querySelector('span');
    const btn = $('qr-renew');
    if (s.status === 'LOGGED_OUT') {
      msg.textContent = 'Nomor ini sudah dikeluarkan dari WhatsApp di HP.';
      btn.textContent = 'Buat kode QR baru';
    } else if (s.phone_number) {
      msg.textContent = 'Koneksi nomor ini terhenti.';
      btn.textContent = 'Sambungkan lagi';
    } else {
      msg.textContent = 'Kode QR sudah tidak berlaku.';
      btn.textContent = 'Buat kode baru';
    }
  }

  function showDone(s) {
    connect.done = true;
    clearInterval(connect.timer);
    connect.timer = null;
    $('connect-flow').hidden = true;
    $('connect-done').hidden = false;
    $('connect-done-phone').textContent = formatPhone(s.phone_number) || s.id;
    setStatus('Nomor siap dipakai untuk mengirim dan menerima pesan.');
    refresh();
  }

  async function tick() {
    const id = connect.id;
    if (!id || connect.busy || connect.done) return;
    connect.busy = true;
    try {
      const s = await api(`${sid(id)}/status`);
      if (connect.id !== id) return;
      const v = view(s);
      if (v.key === 'connected') {
        showDone(s);
      } else if (v.key === 'qr') {
        try {
          const q = await api(`${sid(id)}/qr`);
          if (connect.id !== id) return;
          connect.lastQr = q.qr;
          showQrImage(q.qrImage);
          setStatus('Menunggu kode dipindai…');
        } catch (err) {
          if (err.status !== 404) throw err;
          showQrWait('Menyiapkan kode QR…');
          setStatus('');
        }
      } else if (v.key === 'pending') {
        // Sesudah dipindai, WhatsApp meminta sambung ulang sekali: jangan tampilkan QR lama lagi.
        showQrWait(connect.lastQr ? 'Kode dipindai, menyambungkan…' : 'Menyiapkan kode QR…');
        setStatus('');
      } else {
        showQrExpired(s);
        setStatus('');
      }
    } catch (err) {
      if (err.status === 401) {
        $('dlg-connect').close();
        clearKey();
        showLogin('Kunci master tidak berlaku lagi. Masuk ulang.');
      } else if (err.status === 404) {
        clearInterval(connect.timer);
        showQrWait('Sesi ini sudah dihapus.');
        $('qr-wait').querySelector('.spinner').hidden = true;
      } else {
        setStatus(errorText(err, 'Gagal memeriksa status.'));
      }
    } finally {
      connect.busy = false;
    }
  }

  $('qr-renew').addEventListener('click', async (e) => {
    const btn = e.currentTarget;
    const id = connect.id;
    if (!id) return;
    btn.disabled = true;
    try {
      await api(`${sid(id)}/restart`, { method: 'POST' });
      connect.lastQr = null;
      showQrWait('Menyiapkan kode QR…');
    } catch (err) {
      setStatus(errorText(err, 'Gagal membuat kode baru.'));
    } finally {
      btn.disabled = false;
    }
  });

  function selectTab(which) {
    const qr = which === 'qr';
    $('tab-qr').setAttribute('aria-selected', String(qr));
    $('tab-code').setAttribute('aria-selected', String(!qr));
    $('panel-qr').hidden = !qr;
    $('panel-code').hidden = qr;
  }
  $('tab-qr').addEventListener('click', () => selectTab('qr'));
  $('tab-code').addEventListener('click', () => {
    selectTab('code');
    $('code-phone').focus();
  });

  $('form-code').addEventListener('submit', async (e) => {
    e.preventDefault();
    const id = connect.id;
    const phoneNumber = normalizePhone($('code-phone').value);
    showAlert($('code-error'), '');
    if (!/^\d{10,15}$/.test(phoneNumber)) {
      showAlert($('code-error'), 'Nomor tidak valid. Contoh: 6281234567890 atau 081234567890.');
      return;
    }
    $('code-phone').value = phoneNumber;
    const btn = $('code-submit');
    btn.disabled = true;
    try {
      const data = await api(`${sid(id)}/pairing-code`, { method: 'POST', body: { phoneNumber } });
      const code = String(data.code || '').toUpperCase();
      $('code-value').textContent = code.length === 8 ? `${code.slice(0, 4)}-${code.slice(4)}` : code;
      $('code-result').hidden = false;
    } catch (err) {
      showAlert($('code-error'), errorText(err, 'Gagal meminta kode.'));
    } finally {
      btn.disabled = false;
    }
  });

  // ---------- Dialog: putuskan / hapus ----------
  let removing = null;

  function askRemove(s) {
    removing = s.id;
    const connected = view(s).key === 'connected';
    $('dlg-confirm-title').textContent = connected ? 'Putuskan nomor?' : 'Hapus sesi?';
    $('confirm-ok').textContent = connected ? 'Putuskan' : 'Hapus';
    const who = s.phone_number ? formatPhone(s.phone_number) : 'Nomor yang belum tertaut';
    $('confirm-text').replaceChildren(
      h('strong', { text: who }), ' (sesi ', h('code', { text: s.id }), ') ',
      connected ? 'akan dikeluarkan dari WhatsApp dan hilang dari daftar Perangkat tertaut di HP.' : 'akan dihapus dari daftar.',
    );
    showAlert($('confirm-error'), '');
    $('dlg-confirm').showModal();
  }

  $('confirm-ok').addEventListener('click', async (e) => {
    const btn = e.currentTarget;
    if (!removing) return;
    btn.disabled = true;
    try {
      await api(`${sid(removing)}/logout`, { method: 'DELETE' });
      $('dlg-confirm').close();
      toast($('confirm-ok').textContent === 'Putuskan' ? 'Nomor diputuskan dan sesinya dihapus.' : 'Sesi dihapus.');
      state.lastJson = '';
      refresh();
    } catch (err) {
      showAlert($('confirm-error'), errorText(err, 'Gagal memutuskan.'));
    } finally {
      btn.disabled = false;
      removing = $('dlg-confirm').open ? removing : null;
    }
  });

  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible' && state.key) refresh();
  });

  // ---------- Mulai ----------
  (async () => {
    const key = loadKey();
    if (!key) {
      showLogin('');
      return;
    }
    state.key = key;
    $('view-main').hidden = false;
    $('btn-logout').hidden = false;
    renderSkeleton();
    try {
      renderList(await api('/sessions'));
      showMain();
    } catch (err) {
      if (err.status === 401) {
        clearKey();
        showLogin('Kunci master tidak berlaku lagi. Masuk ulang.');
      } else {
        showMain();
        showAlert($('list-error'), errorText(err, 'Gagal memuat daftar.'));
      }
    }
  })();
})();
