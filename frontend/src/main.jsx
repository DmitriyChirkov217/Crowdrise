import React, { createContext, useContext, useEffect, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { Link, Navigate, Route, BrowserRouter as Router, Routes, useNavigate, useParams } from 'react-router-dom';
import './styles.css';

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';
const WS_URL = API_URL.replace(/^http/, 'ws');
const AuthContext = createContext(null);

function useAuth() {
  return useContext(AuthContext);
}

async function api(path, { method = 'GET', body, token } = {}) {
  const res = await fetch(`${API_URL}${path}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {})
    },
    body: body ? JSON.stringify(body) : undefined
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.message || data.error || 'Request failed');
  return data;
}

function AuthProvider({ children }) {
  const [token, setToken] = useState(localStorage.getItem('token') || '');
  const [user, setUser] = useState(JSON.parse(localStorage.getItem('user') || 'null'));

  function saveAuth(payload) {
    setToken(payload.token);
    setUser(payload.user);
    localStorage.setItem('token', payload.token);
    localStorage.setItem('user', JSON.stringify(payload.user));
  }

  function logout() {
    setToken('');
    setUser(null);
    localStorage.removeItem('token');
    localStorage.removeItem('user');
  }

  return <AuthContext.Provider value={{ token, user, saveAuth, logout }}>{children}</AuthContext.Provider>;
}

function Layout() {
  const auth = useAuth();
  return (
    <>
      <header className="topbar">
        <Link to="/projects" className="brand">Crowdrise</Link>
        <nav>
          <Link to="/projects">Проекты</Link>
          <Link to="/projects/new">Создать</Link>
          {auth.user ? (
            <>
              <Link to="/me">Профиль</Link>
              <button onClick={auth.logout}>Выход</button>
            </>
          ) : (
            <Link to="/login">Вход</Link>
          )}
        </nav>
      </header>
      <main className="shell">
        <Routes>
          <Route path="/" element={<Navigate to="/projects" />} />
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          <Route path="/projects" element={<Projects />} />
          <Route path="/projects/new" element={<ProjectForm />} />
          <Route path="/projects/:id" element={<ProjectDetails />} />
          <Route path="/broadcasts/:id" element={<BroadcastRoom />} />
          <Route path="/projects/:id/edit" element={<ProjectDetails />} />
          <Route path="/projects/:id/milestones" element={<ProjectDetails />} />
          <Route path="/projects/:id/rewards" element={<ProjectDetails />} />
          <Route path="/projects/:id/updates" element={<Updates />} />
          <Route path="/admin/projects" element={<AdminProjects />} />
          <Route path="/admin/projects/:id" element={<AdminProject />} />
          <Route path="/admin/milestones" element={<AdminMilestones />} />
          <Route path="/me" element={<Me />} />
        </Routes>
      </main>
    </>
  );
}

function Login() {
  const auth = useAuth();
  const navigate = useNavigate();
  const [form, setForm] = useState({ email: 'admin@example.com', password: 'admin12345' });
  const [error, setError] = useState('');
  async function submit(e) {
    e.preventDefault();
    try {
      const data = await api('/auth/login', { method: 'POST', body: form });
      auth.saveAuth(data);
      navigate('/projects');
    } catch (err) {
      setError(err.message);
    }
  }
  return <AuthCard title="Вход" form={form} setForm={setForm} submit={submit} error={error} button="Войти" alt={<Link to="/register">Создать аккаунт</Link>} />;
}

function Register() {
  const auth = useAuth();
  const navigate = useNavigate();
  const [form, setForm] = useState({ email: '', password: '', display_name: '' });
  const [error, setError] = useState('');
  async function submit(e) {
    e.preventDefault();
    try {
      const data = await api('/auth/register', { method: 'POST', body: form });
      auth.saveAuth(data);
      navigate('/projects');
    } catch (err) {
      setError(err.message);
    }
  }
  return <AuthCard title="Регистрация" form={form} setForm={setForm} submit={submit} error={error} button="Зарегистрироваться" alt={<Link to="/login">Уже есть аккаунт</Link>} register />;
}

function AuthCard({ title, form, setForm, submit, error, button, alt, register }) {
  return (
    <section className="card narrow">
      <h1>{title}</h1>
      <form onSubmit={submit} className="stack">
        <input placeholder="Email" value={form.email} onChange={e => setForm({ ...form, email: e.target.value })} />
        {register && <input placeholder="Имя" value={form.display_name} onChange={e => setForm({ ...form, display_name: e.target.value })} />}
        <input placeholder="Пароль" type="password" value={form.password} onChange={e => setForm({ ...form, password: e.target.value })} />
        {error && <p className="error">{error}</p>}
        <button>{button}</button>
        {alt}
      </form>
    </section>
  );
}

function Projects() {
  const [projects, setProjects] = useState([]);
  const [categories, setCategories] = useState([]);
  const [campaignType, setCampaignType] = useState('');
  const [categoryID, setCategoryID] = useState('');
  useEffect(() => {
    api(`/projects${campaignType ? `?campaign_type=${campaignType}` : ''}`).then(data => setProjects(data.items || []));
  }, [campaignType]);
  useEffect(() => {
    api('/categories').then(setCategories);
  }, []);
  const visibleProjects = projects
    .filter(project => !categoryID || String(project.category_id || '') === categoryID)
    .sort((a, b) => ((b.funds?.total_collected || 0) - (a.funds?.total_collected || 0)));
  return (
    <section className="stack">
      <section className="filter-panel">
        <button className={campaignType === '' ? 'active' : ''} onClick={() => setCampaignType('')}>Все</button>
        <button className={campaignType === 'reward' ? 'active' : ''} onClick={() => setCampaignType('reward')}>Reward</button>
        <button className={campaignType === 'donation' ? 'active' : ''} onClick={() => setCampaignType('donation')}>Donation</button>
      </section>
      <section className="filter-panel category-panel">
        <button className={categoryID === '' ? 'active' : ''} onClick={() => setCategoryID('')}>Все категории</button>
        {categories.map(category => (
          <button className={categoryID === String(category.id) ? 'active' : ''} key={category.id} onClick={() => setCategoryID(String(category.id))}>{category.name}</button>
        ))}
      </section>
      <section className="carousel-section">
        <p className="eyebrow">Самые поддерживаемые проекты</p>
        <div className="project-carousel">
          {visibleProjects.map(project => <ProjectCard key={project.id} project={project} />)}
          {!visibleProjects.length && <p className="muted">Проектов пока нет</p>}
        </div>
      </section>
    </section>
  );
  return (
    <section>
      <div className="hero">
        <p className="eyebrow">MVP с этапным доступом к средствам</p>
        <h1>Проекты, где деньги движутся только после подтверждённых этапов</h1>
        <div className="filters">
          <button onClick={() => setCampaignType('')}>Все</button>
          <button onClick={() => setCampaignType('reward')}>Reward</button>
          <button onClick={() => setCampaignType('donation')}>Donation</button>
        </div>
      </div>
      <div className="grid">
        {projects.map(project => <ProjectCard key={project.id} project={project} />)}
      </div>
    </section>
  );
}

function ProjectCard({ project }) {
  const funds = project.funds || {};
  const progress = Math.min(100, Math.round(((funds.total_collected || 0) / project.goal_amount) * 100));
  return (
    <Link to={`/projects/${project.id}`} className="card project-card">
      <span className="pill">{project.campaign_type}</span>
      <h2>{project.title}</h2>
      <p>{project.short_description}</p>
      <div className="progress"><span style={{ width: `${progress}%` }} /></div>
      <strong>{money(funds.total_collected)} из {money(project.goal_amount)}</strong>
    </Link>
  );
}

function ProjectDetails() {
  const { id } = useParams();
  const auth = useAuth();
  const navigate = useNavigate();
  const [data, setData] = useState(null);
  const [pledge, setPledge] = useState({ amount: 1000, reward_id: null });
  const [mediaForm, setMediaForm] = useState({ media_type: 'image', url: '', sort_order: 0 });
  const [report, setReport] = useState({ milestone_id: '', report_text: '', files: [{ file_url: '', file_type: 'document' }] });
  const [paymentId, setPaymentId] = useState('');
  const [message, setMessage] = useState('');
  const load = () => api(`/projects/${id}`).then(setData);
  useEffect(load, [id]);
  if (!data) return <p>Загрузка...</p>;
  const p = data.project;
  const funds = p.funds || {};
  const milestones = asArray(data.milestones);
  const rewards = asArray(data.rewards);
  const updates = asArray(data.updates);
  const media = asArray(data.media);
  const broadcasts = asArray(data.broadcasts);
  const activeBroadcast = broadcasts.find(room => room.status === 'live')
    || broadcasts.find(room => room.status === 'scheduled')
    || broadcasts[0];
  const isAuthor = auth.user?.id === p.author_id;
  const isAdmin = auth.user?.roles?.includes('admin');
  async function submitForReview() {
    try {
      const res = await api(`/projects/${id}/submit`, { method: 'POST', token: auth.token, body: {} });
      setMessage(`Проект отправлен на модерацию: ${res.status}`);
      load();
    } catch (err) {
      setMessage(err.message);
    }
  }
  async function support(e) {
    e.preventDefault();
    try {
      const res = await api(`/projects/${id}/pledges`, { method: 'POST', token: auth.token, body: { ...pledge, amount: Number(pledge.amount), reward_id: pledge.reward_id || null } });
      setPaymentId(res.payment_id);
      setMessage(`Поддержка создана. Payment ID: ${res.payment_id}`);
      load();
    } catch (err) {
      setMessage(err.message);
    }
  }
  async function capturePayment() {
    const idToCapture = paymentId || window.prompt('Payment ID');
    if (!idToCapture) return;
    try {
      const res = await api(`/payments/${idToCapture}/mock-capture`, { method: 'POST', token: auth.token, body: {} });
      setMessage(`Платёж обработан: ${res.status}`);
      setPaymentId('');
      load();
    } catch (err) {
      setMessage(err.message);
    }
  }
  async function addMedia(e) {
    e.preventDefault();
    try {
      await api(`/projects/${id}/media`, { method: 'POST', token: auth.token, body: { ...mediaForm, sort_order: Number(mediaForm.sort_order) } });
      setMediaForm({ media_type: 'image', url: '', sort_order: media.length + 1 });
      setMessage('Медиа добавлено');
      load();
    } catch (err) {
      setMessage(err.message);
    }
  }
  async function submitMilestoneReport(e) {
    e.preventDefault();
    try {
      const files = report.files
        .filter(file => file.file_url.trim())
        .map(file => ({ file_url: file.file_url.trim(), file_type: file.file_type || 'document' }));
      const res = await api(`/milestones/${report.milestone_id}/submit`, {
        method: 'POST',
        token: auth.token,
        body: { report_text: report.report_text, files }
      });
      setReport({ milestone_id: '', report_text: '', files: [{ file_url: '', file_type: 'document' }] });
      setMessage(`Отчёт отправлен: ${res.submission_id}`);
      load();
    } catch (err) {
      setMessage(err.message);
    }
  }
  async function createBroadcast() {
    try {
      const room = await api(`/projects/${id}/broadcasts`, { method: 'POST', token: auth.token, body: { status: 'scheduled' } });
      setMessage('Broadcast room created');
      load();
      return room;
    } catch (err) {
      setMessage(err.message);
      return null;
    }
  }
  async function openBroadcastChat() {
    if (activeBroadcast) {
      navigate(`/broadcasts/${activeBroadcast.id}`);
      return;
    }
    if (!isAuthor) {
      return;
    }
    const room = await createBroadcast();
    if (room?.id) {
      navigate(`/broadcasts/${room.id}`);
    }
  }
  async function setBroadcastStatus(broadcastID, status) {
    try {
      await api(`/broadcasts/${broadcastID}/status`, { method: 'PUT', token: auth.token, body: { status } });
      setMessage(`Broadcast status: ${status}`);
      load();
    } catch (err) {
      setMessage(err.message);
    }
  }
  function updateReportFile(index, patch) {
    setReport(current => ({ ...current, files: current.files.map((file, i) => i === index ? { ...file, ...patch } : file) }));
  }
  return (
    <section className="stack">
      <article className="card">
        <span className="pill">{p.status} · {p.campaign_type}</span>
        <h1>{p.title}</h1>
        <p className="lead">{p.short_description}</p>
        <p>{p.description}</p>
        <div className="inline">
          {(activeBroadcast || isAuthor) && <button onClick={openBroadcastChat}>Broadcast chat</button>}
          {isAuthor && <Link className="button-link" to={`/projects/${id}/updates`}>Объявления</Link>}
          {isAuthor && (p.status === 'draft' || p.status === 'rejected') && <button onClick={submitForReview}>На модерацию</button>}
          {isAdmin && <button onClick={capturePayment}>Подтвердить оплату</button>}
        </div>
        <div className="stats">
          <Metric label="Цель" value={money(p.goal_amount)} />
          <Metric label="Собрано" value={money(funds.total_collected)} />
          <Metric label="Зарезервировано" value={money(funds.total_reserved)} />
          <Metric label="Доступно" value={money(funds.total_available)} />
          <Metric label="Возвращено" value={money(funds.total_refunded)} />
        </div>
      </article>
      <section className="card stack">
        <div className="inline between">
          <h2>Broadcast rooms</h2>
          {isAuthor && <button onClick={createBroadcast}>Create room</button>}
        </div>
        {broadcasts.length ? <ul className="list">
          {broadcasts.map(room => (
            <li key={room.id}>
              <div className="stack">
                <div className="inline">
                  <span>{room.status}</span>
                  <Link className="button-link secondary" to={`/broadcasts/${room.id}`}>Open voice room</Link>
                </div>
                {isAuthor && <div className="inline">
                  <button onClick={() => setBroadcastStatus(room.id, 'scheduled')}>Scheduled</button>
                  <button onClick={() => setBroadcastStatus(room.id, 'live')}>Live</button>
                  <button onClick={() => setBroadcastStatus(room.id, 'ended')}>Ended</button>
                </div>}
                <FileLinks files={asArray(room.files).map(file => ({ file_url: file.file_url, file_type: 'document' }))} />
              </div>
            </li>
          ))}
        </ul> : <p className="muted">No broadcast rooms yet</p>}
      </section>
      <MilestoneTimeline milestones={milestones} />
      <MediaGallery media={media} />
      {isAuthor && <section className="card">
        <h2>Медиа проекта</h2>
        <form className="stack" onSubmit={addMedia}>
          <select value={mediaForm.media_type} onChange={e => setMediaForm({ ...mediaForm, media_type: e.target.value })}>
            <option value="image">Изображение</option>
            <option value="video">Видео</option>
            <option value="document">Документ</option>
          </select>
          <input placeholder="URL файла" value={mediaForm.url} onChange={e => setMediaForm({ ...mediaForm, url: e.target.value })} />
          <input type="number" placeholder="Порядок" value={mediaForm.sort_order} onChange={e => setMediaForm({ ...mediaForm, sort_order: e.target.value })} />
          <button>Добавить медиа</button>
        </form>
      </section>}
      <div className="two-col">
        <Panel title="Этапы" items={milestones} render={m => `${m.position}. ${m.title} · ${money(m.amount_limit)} · ${m.status}`} />
        <Panel title="Вознаграждения" items={rewards} render={r => `${r.title} от ${money(r.min_amount)}`} />
      </div>
      {isAuthor && <section className="card">
        <h2>Отчёт по этапу</h2>
        <form className="stack" onSubmit={submitMilestoneReport}>
          <select value={report.milestone_id} onChange={e => setReport({ ...report, milestone_id: e.target.value })}>
            <option value="">Выберите этап</option>
            {milestones.map(m => <option key={m.id} value={m.id}>{m.position}. {m.title}</option>)}
          </select>
          <textarea placeholder="Что сделано по этапу" value={report.report_text} onChange={e => setReport({ ...report, report_text: e.target.value })} />
          {report.files.map((file, index) => (
            <div className="inline" key={index}>
              <select value={file.file_type} onChange={e => updateReportFile(index, { file_type: e.target.value })}>
                <option value="document">Документ</option>
                <option value="image">Изображение</option>
                <option value="video">Видео</option>
              </select>
              <input placeholder="URL файла отчёта" value={file.file_url} onChange={e => updateReportFile(index, { file_url: e.target.value })} />
              {report.files.length > 1 && <button type="button" onClick={() => setReport(current => ({ ...current, files: current.files.filter((_, i) => i !== index) }))}>Удалить</button>}
            </div>
          ))}
          <button type="button" onClick={() => setReport(current => ({ ...current, files: [...current.files, { file_url: '', file_type: 'document' }] }))}>Добавить файл</button>
          <button disabled={!report.milestone_id}>Отправить отчёт</button>
        </form>
      </section>}
      <section className="card">
        <h2>Поддержать проект</h2>
        <form className="inline" onSubmit={support}>
          <input type="number" value={pledge.amount} onChange={e => setPledge({ ...pledge, amount: e.target.value })} />
          <select value={pledge.reward_id || ''} onChange={e => setPledge({ ...pledge, reward_id: e.target.value || null })}>
            <option value="">Без вознаграждения</option>
            {rewards.map(r => <option key={r.id} value={r.id}>{r.title}</option>)}
          </select>
          <button disabled={!auth.token}>Поддержать</button>
        </form>
        {message && <p className="notice">{message}</p>}
      </section>
      <Panel title="Объявления" items={updates} render={u => `${u.title}: ${u.content}`} />
    </section>
  );
}

function BroadcastRoom() {
  const { id } = useParams();
  const auth = useAuth();
  const [files, setFiles] = useState([]);
  const [fileURL, setFileURL] = useState('');
  const [connected, setConnected] = useState(false);
  const [muted, setMuted] = useState(false);
  const [peerId, setPeerId] = useState('');
  const [remoteStreams, setRemoteStreams] = useState({});
  const [notice, setNotice] = useState('');
  const wsRef = useRef(null);
  const pcsRef = useRef({});
  const localStreamRef = useRef(null);

  const loadFiles = () => api(`/broadcasts/${id}/files`).then(data => setFiles(data.items || []));
  useEffect(() => {
    loadFiles();
    return () => disconnect();
  }, [id]);

  function send(message) {
    const ws = wsRef.current;
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(message));
    }
  }

  async function createPeer(remotePeerID, initiator) {
    if (pcsRef.current[remotePeerID]) return pcsRef.current[remotePeerID];
    const pc = new RTCPeerConnection({
      iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
    });
    pcsRef.current[remotePeerID] = pc;
    localStreamRef.current?.getTracks().forEach(track => pc.addTrack(track, localStreamRef.current));
    pc.onicecandidate = event => {
      if (event.candidate) {
        send({ type: 'ice_candidate', to: remotePeerID, payload: event.candidate });
      }
    };
    pc.ontrack = event => {
      const [stream] = event.streams;
      if (stream) {
        setRemoteStreams(current => ({ ...current, [remotePeerID]: stream }));
      }
    };
    pc.onconnectionstatechange = () => {
      if (['failed', 'closed', 'disconnected'].includes(pc.connectionState)) {
        closePeer(remotePeerID);
      }
    };
    if (initiator) {
      const offer = await pc.createOffer();
      await pc.setLocalDescription(offer);
      send({ type: 'offer', to: remotePeerID, payload: offer });
    }
    return pc;
  }

  function closePeer(remotePeerID) {
    pcsRef.current[remotePeerID]?.close();
    delete pcsRef.current[remotePeerID];
    setRemoteStreams(current => {
      const next = { ...current };
      delete next[remotePeerID];
      return next;
    });
  }

  async function connect() {
    if (!auth.token) {
      setNotice('Login is required to join voice');
      return;
    }
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      localStreamRef.current = stream;
      const ws = new WebSocket(`${WS_URL}/broadcasts/${id}/ws?token=${encodeURIComponent(auth.token)}`);
      wsRef.current = ws;
      ws.onopen = () => {
        setConnected(true);
        setNotice('Voice room connected');
      };
      ws.onclose = () => {
        setConnected(false);
        setNotice('Voice room disconnected');
      };
      ws.onerror = () => setNotice('Voice connection failed');
      ws.onmessage = async event => {
        const msg = JSON.parse(event.data);
        if (msg.type === 'joined') {
          setPeerId(msg.peer_id);
          for (const peer of msg.peers || []) {
            await createPeer(peer.peer_id, true);
          }
        }
        if (msg.type === 'peer_joined') {
          await createPeer(msg.peer_id, false);
        }
        if (msg.type === 'offer') {
          const pc = await createPeer(msg.from, false);
          await pc.setRemoteDescription(new RTCSessionDescription(msg.payload));
          const answer = await pc.createAnswer();
          await pc.setLocalDescription(answer);
          send({ type: 'answer', to: msg.from, payload: answer });
        }
        if (msg.type === 'answer') {
          await pcsRef.current[msg.from]?.setRemoteDescription(new RTCSessionDescription(msg.payload));
        }
        if (msg.type === 'ice_candidate') {
          await pcsRef.current[msg.from]?.addIceCandidate(new RTCIceCandidate(msg.payload));
        }
        if (msg.type === 'peer_left') {
          closePeer(msg.peer_id);
        }
      };
    } catch (err) {
      setNotice(err.message);
    }
  }

  function disconnect() {
    send({ type: 'leave' });
    wsRef.current?.close();
    Object.keys(pcsRef.current).forEach(closePeer);
    localStreamRef.current?.getTracks().forEach(track => track.stop());
    localStreamRef.current = null;
    setConnected(false);
    setPeerId('');
  }

  function toggleMute() {
    const nextMuted = !muted;
    localStreamRef.current?.getAudioTracks().forEach(track => {
      track.enabled = !nextMuted;
    });
    setMuted(nextMuted);
    send({ type: 'mute', muted: nextMuted });
  }

  async function addFile(e) {
    e.preventDefault();
    try {
      await api(`/broadcasts/${id}/files`, { method: 'POST', token: auth.token, body: { file_url: fileURL } });
      setFileURL('');
      setNotice('File added');
      loadFiles();
    } catch (err) {
      setNotice(err.message);
    }
  }

  return (
    <section className="stack">
      <section className="card stack">
        <div className="inline between">
          <h1>Broadcast voice room</h1>
          <Link className="button-link secondary" to="/projects">Projects</Link>
        </div>
        <p className="muted">Peer ID: {peerId || 'not connected'}</p>
        <div className="inline">
          {!connected ? <button onClick={connect} disabled={!auth.token}>Join with microphone</button> : <button onClick={disconnect}>Leave</button>}
          <button onClick={toggleMute} disabled={!connected}>{muted ? 'Unmute' : 'Mute'}</button>
        </div>
        {notice && <p className="notice">{notice}</p>}
      </section>
      <section className="card stack">
        <h2>Voice peers</h2>
        <LocalAudio stream={localStreamRef.current} />
        {Object.entries(remoteStreams).map(([id, stream]) => <RemoteAudio key={id} peerID={id} stream={stream} />)}
        {!Object.keys(remoteStreams).length && <p className="muted">No remote speakers yet</p>}
      </section>
      <section className="card stack">
        <h2>Broadcast files</h2>
        <FileLinks files={files.map(file => ({ file_url: file.file_url, file_type: 'document' }))} />
        <form className="inline" onSubmit={addFile}>
          <input placeholder="File URL" value={fileURL} onChange={e => setFileURL(e.target.value)} />
          <button disabled={!auth.token || !fileURL.trim()}>Add file</button>
        </form>
      </section>
    </section>
  );
}

function LocalAudio({ stream }) {
  const ref = useRef(null);
  useEffect(() => {
    if (ref.current) ref.current.srcObject = stream;
  }, [stream]);
  if (!stream) return null;
  return <audio ref={ref} autoPlay muted />;
}

function RemoteAudio({ peerID, stream }) {
  const ref = useRef(null);
  useEffect(() => {
    if (ref.current) ref.current.srcObject = stream;
  }, [stream]);
  return <div className="stack embedded">
    <span>{peerID}</span>
    <audio ref={ref} autoPlay controls />
  </div>;
}

function ProjectForm({ edit }) {
  const { id } = useParams();
  const auth = useAuth();
  const navigate = useNavigate();
  const [categories, setCategories] = useState([]);
  const [form, setForm] = useState({ title: '', short_description: '', description: '', category_id: 1, campaign_type: 'reward', currency: 'RUB', goal_amount: 100000 });
  const [milestones, setMilestones] = useState([{ title: '', description: '', due_at: '2026-07-01T00:00:00Z', amount_limit: 100000, position: 1 }]);
  const [rewards, setRewards] = useState([{ title: '', description: '', min_amount: 1000, limit_count: 100, delivery_estimate: '2026-09-01' }]);
  const [media, setMedia] = useState([{ media_type: 'image', url: '', sort_order: 1 }]);
  const [error, setError] = useState('');
  useEffect(() => { api('/categories').then(setCategories); }, []);
  useEffect(() => {
    if (edit && id) api(`/projects/${id}`).then(data => setForm({ ...data.project, category_id: data.project.category_id || 1 }));
  }, [edit, id]);
  async function submit(e) {
    e.preventDefault();
    try {
      const body = { ...form, goal_amount: Number(form.goal_amount), category_id: Number(form.category_id) };
      const data = await api(edit ? `/projects/${id}` : '/projects', { method: edit ? 'PUT' : 'POST', token: auth.token, body });
      if (!edit) {
        for (const [index, milestone] of milestones.entries()) {
          if (!milestone.title.trim()) continue;
          await api(`/projects/${data.id}/milestones`, {
            method: 'POST',
            token: auth.token,
            body: {
              ...milestone,
              amount_limit: Number(milestone.amount_limit),
              position: Number(milestone.position || index + 1)
            }
          });
        }
        if (form.campaign_type === 'reward') {
          for (const reward of rewards) {
            if (!reward.title.trim()) continue;
            await api(`/projects/${data.id}/rewards`, {
              method: 'POST',
              token: auth.token,
              body: {
                ...reward,
                min_amount: Number(reward.min_amount),
                limit_count: reward.limit_count === '' ? null : Number(reward.limit_count)
              }
            });
          }
        }
        for (const [index, item] of media.entries()) {
          if (!item.url.trim()) continue;
          await api(`/projects/${data.id}/media`, {
            method: 'POST',
            token: auth.token,
            body: {
              media_type: item.media_type,
              url: item.url.trim(),
              sort_order: Number(item.sort_order || index + 1)
            }
          });
        }
      }
      navigate(`/projects/${data.id}`);
    } catch (err) {
      setError(err.message);
    }
  }

  function updateMilestone(index, patch) {
    setMilestones(items => items.map((item, i) => i === index ? { ...item, ...patch } : item));
  }

  function updateReward(index, patch) {
    setRewards(items => items.map((item, i) => i === index ? { ...item, ...patch } : item));
  }

  function updateMedia(index, patch) {
    setMedia(items => items.map((item, i) => i === index ? { ...item, ...patch } : item));
  }
  return (
    <section className="card">
      <h1>{edit ? 'Редактировать проект' : 'Новый проект'}</h1>
      <form onSubmit={submit} className="stack">
        <label className="field-label">Название проекта</label>
        <input placeholder="Название" value={form.title} onChange={e => setForm({ ...form, title: e.target.value })} />
        <label className="field-label">Короткое описание</label>
        <input placeholder="Краткое описание" value={form.short_description} onChange={e => setForm({ ...form, short_description: e.target.value })} />
        <label className="field-label">Полное описание</label>
        <textarea placeholder="Полное описание" value={form.description} onChange={e => setForm({ ...form, description: e.target.value })} />
        <label className="field-label">Категория</label>
        <select value={form.category_id} onChange={e => setForm({ ...form, category_id: e.target.value })}>{categories.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}</select>
        <label className="field-label">Тип проекта</label>
        <select value={form.campaign_type} onChange={e => setForm({ ...form, campaign_type: e.target.value })}><option value="reward">Reward</option><option value="donation">Donation</option></select>
        <label className="field-label">Валюта</label>
        <input value={form.currency} onChange={e => setForm({ ...form, currency: e.target.value.toUpperCase() })} />
        <label className="field-label">Целевая сумма</label>
        <input type="number" value={form.goal_amount} onChange={e => setForm({ ...form, goal_amount: e.target.value })} />
        {!edit && <section className="stack">
          <h2>Этапы</h2>
          {milestones.map((milestone, index) => (
            <div className="stack embedded" key={index}>
              <label className="field-label">Название этапа</label>
              <input placeholder="Название этапа" value={milestone.title} onChange={e => updateMilestone(index, { title: e.target.value })} />
              <label className="field-label">Описание этапа</label>
              <textarea placeholder="Описание этапа" value={milestone.description} onChange={e => updateMilestone(index, { description: e.target.value })} />
              <label className="field-label">Дата этапа</label>
              <input placeholder="Дата этапа" value={milestone.due_at} onChange={e => updateMilestone(index, { due_at: e.target.value })} />
              <label className="field-label">Сумма этапа</label>
              <input type="number" placeholder="Сумма этапа" value={milestone.amount_limit} onChange={e => updateMilestone(index, { amount_limit: e.target.value })} />
              <label className="field-label">Порядковый номер этапа</label>
              <input type="number" placeholder="Позиция" value={milestone.position} onChange={e => updateMilestone(index, { position: e.target.value })} />
              {milestones.length > 1 && <button type="button" onClick={() => setMilestones(items => items.filter((_, i) => i !== index))}>Удалить этап</button>}
            </div>
          ))}
          <button type="button" onClick={() => setMilestones(items => [...items, { title: '', description: '', due_at: '2026-07-01T00:00:00Z', amount_limit: 10000, position: items.length + 1 }])}>Добавить этап</button>
        </section>}
        {!edit && form.campaign_type === 'reward' && <section className="stack">
          <h2>Вознаграждения</h2>
          {rewards.map((reward, index) => (
            <div className="stack embedded" key={index}>
              <label className="field-label">Название вознаграждения</label>
              <input placeholder="Название вознаграждения" value={reward.title} onChange={e => updateReward(index, { title: e.target.value })} />
              <label className="field-label">Описание вознаграждения</label>
              <textarea placeholder="Описание вознаграждения" value={reward.description} onChange={e => updateReward(index, { description: e.target.value })} />
              <label className="field-label">Минимальная сумма поддержки</label>
              <input type="number" placeholder="Минимальная сумма" value={reward.min_amount} onChange={e => updateReward(index, { min_amount: e.target.value })} />
              <label className="field-label">Лимит вознаграждений</label>
              <input type="number" placeholder="Лимит" value={reward.limit_count} onChange={e => updateReward(index, { limit_count: e.target.value })} />
              <label className="field-label">Ожидаемая дата доставки</label>
              <input placeholder="Дата доставки" value={reward.delivery_estimate} onChange={e => updateReward(index, { delivery_estimate: e.target.value })} />
              {rewards.length > 1 && <button type="button" onClick={() => setRewards(items => items.filter((_, i) => i !== index))}>Удалить вознаграждение</button>}
            </div>
          ))}
          <button type="button" onClick={() => setRewards(items => [...items, { title: '', description: '', min_amount: 1000, limit_count: 100, delivery_estimate: '2026-09-01' }])}>Добавить вознаграждение</button>
        </section>}
        {!edit && <section className="stack">
          <h2>Медиа</h2>
          {media.map((item, index) => (
            <div className="stack embedded" key={index}>
              <label className="field-label">Тип медиа</label>
              <select value={item.media_type} onChange={e => updateMedia(index, { media_type: e.target.value })}>
                <option value="image">Изображение</option>
                <option value="video">Видео</option>
                <option value="document">Документ</option>
              </select>
              <label className="field-label">Ссылка на файл</label>
              <input placeholder="URL файла" value={item.url} onChange={e => updateMedia(index, { url: e.target.value })} />
              <label className="field-label">Порядок отображения</label>
              <input type="number" placeholder="Порядок" value={item.sort_order} onChange={e => updateMedia(index, { sort_order: e.target.value })} />
              {media.length > 1 && <button type="button" onClick={() => setMedia(items => items.filter((_, i) => i !== index))}>Удалить медиа</button>}
            </div>
          ))}
          <button type="button" onClick={() => setMedia(items => [...items, { media_type: 'image', url: '', sort_order: items.length + 1 }])}>Добавить медиа</button>
        </section>}
        {error && <p className="error">{error}</p>}
        <button>{edit ? 'Сохранить' : 'Создать'}</button>
      </form>
    </section>
  );
}

function Milestones() {
  const { id } = useParams();
  const auth = useAuth();
  const [project, setProject] = useState(null);
  const [form, setForm] = useState({ title: '', description: '', due_at: '2026-07-01T00:00:00Z', amount_limit: 10000, position: 1 });
  const [editingId, setEditingId] = useState('');
  const [report, setReport] = useState({ milestone_id: '', report_text: '' });
  const [message, setMessage] = useState('');
  const load = () => api(`/projects/${id}`).then(setProject);
  useEffect(load, [id]);
  async function save(e) {
    e.preventDefault();
    const path = editingId ? `/projects/${id}/milestones/${editingId}` : `/projects/${id}/milestones`;
    await api(path, { method: editingId ? 'PUT' : 'POST', token: auth.token, body: { ...form, amount_limit: Number(form.amount_limit), position: Number(form.position) } });
    setMessage(editingId ? 'Этап обновлён' : 'Этап добавлен');
    setEditingId('');
    setForm({ title: '', description: '', due_at: '2026-07-01T00:00:00Z', amount_limit: 10000, position: 1 });
    await load();
  }
  function editMilestone(m) {
    setEditingId(m.id);
    setForm({ title: m.title, description: m.description, due_at: m.due_at, amount_limit: m.amount_limit, position: m.position });
  }
  async function submitReport(e) {
    e.preventDefault();
    const res = await api(`/milestones/${report.milestone_id}/submit`, { method: 'POST', token: auth.token, body: { report_text: report.report_text, files: [] } });
    setMessage(`Отчёт отправлен: ${res.submission_id}`);
    load();
  }
  return (
    <section className="stack">
      <Panel title="Текущие этапы" items={project?.milestones || []} render={m => <span>{m.id} · {m.title} · {money(m.amount_limit)} · {m.status} <button onClick={() => editMilestone(m)}>Редактировать</button></span>} />
      <FormCard title={editingId ? 'Редактировать этап' : 'Добавить этап'} onSubmit={save}>
        <input placeholder="Название" value={form.title} onChange={e => setForm({ ...form, title: e.target.value })} />
        <textarea placeholder="Описание" value={form.description} onChange={e => setForm({ ...form, description: e.target.value })} />
        <input value={form.due_at} onChange={e => setForm({ ...form, due_at: e.target.value })} />
        <input type="number" value={form.amount_limit} onChange={e => setForm({ ...form, amount_limit: e.target.value })} />
        <input type="number" value={form.position} onChange={e => setForm({ ...form, position: e.target.value })} />
      </FormCard>
      <FormCard title="Отправить отчёт" onSubmit={submitReport}>
        <select value={report.milestone_id} onChange={e => setReport({ ...report, milestone_id: e.target.value })}>
          <option value="">Выберите этап</option>
          {(project?.milestones || []).map(m => <option key={m.id} value={m.id}>{m.title}</option>)}
        </select>
        <textarea placeholder="Текст отчёта" value={report.report_text} onChange={e => setReport({ ...report, report_text: e.target.value })} />
        {message && <p className="notice">{message}</p>}
      </FormCard>
    </section>
  );
}

function Rewards() {
  const { id } = useParams();
  const auth = useAuth();
  const [project, setProject] = useState(null);
  const [form, setForm] = useState({ title: '', description: '', min_amount: 1500, limit_count: 100, delivery_estimate: '2026-09-01' });
  const load = () => api(`/projects/${id}`).then(setProject);
  useEffect(load, [id]);
  async function add(e) {
    e.preventDefault();
    await api(`/projects/${id}/rewards`, { method: 'POST', token: auth.token, body: { ...form, min_amount: Number(form.min_amount), limit_count: Number(form.limit_count) } });
    load();
  }
  return (
    <section className="stack">
      <Panel title="Вознаграждения" items={project?.rewards || []} render={r => `${r.title} · от ${money(r.min_amount)}`} />
      <FormCard title="Добавить вознаграждение" onSubmit={add}>
        <input placeholder="Название" value={form.title} onChange={e => setForm({ ...form, title: e.target.value })} />
        <textarea placeholder="Описание" value={form.description} onChange={e => setForm({ ...form, description: e.target.value })} />
        <input type="number" value={form.min_amount} onChange={e => setForm({ ...form, min_amount: e.target.value })} />
        <input type="number" value={form.limit_count} onChange={e => setForm({ ...form, limit_count: e.target.value })} />
        <input value={form.delivery_estimate} onChange={e => setForm({ ...form, delivery_estimate: e.target.value })} />
      </FormCard>
    </section>
  );
}

function Updates() {
  const { id } = useParams();
  const auth = useAuth();
  const [project, setProject] = useState(null);
  const [form, setForm] = useState({ title: '', content: '' });
  const [message, setMessage] = useState('');
  useEffect(() => { api(`/projects/${id}`).then(setProject); }, [id]);
  async function submit(e) {
    e.preventDefault();
    await api(`/projects/${id}/updates`, { method: 'POST', token: auth.token, body: form });
    setMessage('Обновление опубликовано');
  }
  if (!auth.token) return <Navigate to="/login" />;
  if (!project) return <p>Загрузка...</p>;
  if (project.project.author_id !== auth.user?.id) return <p>Объявления может публиковать только автор проекта.</p>;
  return <FormCard title="Опубликовать объявление" onSubmit={submit}>
    <input placeholder="Заголовок" value={form.title} onChange={e => setForm({ ...form, title: e.target.value })} />
    <textarea placeholder="Текст объявления" value={form.content} onChange={e => setForm({ ...form, content: e.target.value })} />
    {message && <p className="notice">{message}</p>}
  </FormCard>;
}

function AdminProjects() {
  const auth = useAuth();
  const [projects, setProjects] = useState([]);
  const [userID, setUserID] = useState('');
  const [message, setMessage] = useState('');
  useEffect(() => { api('/admin/projects?status=on_review', { token: auth.token }).then(data => setProjects(data.items || [])).catch(() => setProjects([])); }, [auth.token]);
  async function setBlocked(blocked) {
    try {
      const action = blocked ? 'block' : 'unblock';
      const res = await api(`/admin/users/${userID}/${action}`, { method: 'POST', token: auth.token, body: {} });
      setMessage(`Пользователь обновлён: is_blocked=${res.is_blocked}`);
    } catch (err) {
      setMessage(err.message);
    }
  }
  return <section className="stack">
    <Panel title="Проекты на модерации" items={projects} render={p => <Link to={`/admin/projects/${p.id}`}>{p.title} · {p.status}</Link>} />
    <section className="card">
      <h2>Пользователи</h2>
      <div className="inline">
        <input placeholder="User ID" value={userID} onChange={e => setUserID(e.target.value)} />
        <button disabled={!userID} onClick={() => setBlocked(true)}>Заблокировать</button>
        <button disabled={!userID} onClick={() => setBlocked(false)}>Разблокировать</button>
      </div>
      {message && <p className="notice">{message}</p>}
    </section>
  </section>;
}

function AdminProject() {
  const { id } = useParams();
  const auth = useAuth();
  const [data, setData] = useState(null);
  const [message, setMessage] = useState('');
  useEffect(() => { api(`/admin/projects/${id}`, { token: auth.token }).then(setData); }, [id, auth.token]);
  async function decide(decision) {
    await api(`/admin/projects/${id}/decision`, { method: 'POST', token: auth.token, body: { decision, comment: decision } });
    setMessage(`Решение применено: ${decision}`);
  }
  if (!data) return <p>Загрузка...</p>;
  return <section className="card stack">
    <h1>{data.project.title}</h1>
    <p>{data.project.description}</p>
    <div className="inline">
      <button onClick={() => decide('approved')}>Одобрить</button>
      <button onClick={() => decide('rejected')}>Отклонить</button>
      <button onClick={() => decide('blocked')}>Заблокировать</button>
    </div>
    {message && <p className="notice">{message}</p>}
  </section>;
}

function AdminMilestones() {
  const auth = useAuth();
  const [items, setItems] = useState([]);
  const [message, setMessage] = useState('');
  const load = () => api('/admin/milestones', { token: auth.token }).then(data => setItems(data.items || []));
  useEffect(load, [auth.token]);
  async function review(item, decision) {
    await api(`/admin/milestones/${item.milestone_id}/review`, { method: 'POST', token: auth.token, body: { submission_id: item.id, decision, comment: decision } });
    setMessage(`Решение: ${decision}`);
    load();
  }
  return <section className="stack">
    <Panel title="Отчёты по этапам" items={items} render={item => <div className="stack">
      <span>{item.project_title} · {item.milestone_title} · {item.report_text}</span>
      <FileLinks files={item.files} />
      <div className="inline">
        <button onClick={() => review(item, 'approved')}>Одобрить</button>
        <button onClick={() => review(item, 'rejected')}>Отклонить</button>
      </div>
    </div>} />
    {message && <p className="notice">{message}</p>}
  </section>;
}

function Me() {
  const auth = useAuth();
  const [me, setMe] = useState(null);
  useEffect(() => { if (auth.token) api('/users/me', { token: auth.token }).then(setMe); }, [auth.token]);
  if (!auth.token) return <Navigate to="/login" />;
  return <section className="card"><h1>Профиль</h1><pre>{JSON.stringify(me || auth.user, null, 2)}</pre></section>;
}

function Panel({ title, items = [], render }) {
  return <section className="card"><h2>{title}</h2>{items.length ? <ul className="list">{items.map((item, i) => <li key={item.id || i}>{render(item)}</li>)}</ul> : <p className="muted">Пока пусто</p>}</section>;
}

function FormCard({ title, onSubmit, children }) {
  return <section className="card"><h2>{title}</h2><form onSubmit={onSubmit} className="stack">{children}<button>Сохранить</button></form></section>;
}

function Metric({ label, value }) {
  return <div><span>{label}</span><strong>{value}</strong></div>;
}

function MediaGallery({ media }) {
  const items = asArray(media);
  if (!items.length) return null;
  return <section className="card">
    <h2>Медиа</h2>
    <div className="media-grid">
      {items.map((item, index) => <a className="media-item" key={item.id || index} href={item.url} target="_blank" rel="noreferrer">
        {item.media_type === 'image' ? <img src={item.url} alt="" /> : <span>{mediaLabel(item.media_type)}</span>}
      </a>)}
    </div>
  </section>;
}

function MilestoneTimeline({ milestones }) {
  const items = asArray(milestones).sort((a, b) => (a.position || 0) - (b.position || 0));
  if (!items.length) return null;
  const doneCount = items.filter(item => item.status === 'approved').length;
  const progress = (doneCount / items.length) * 100;
  return <section className="card">
    <h2>Шкала этапов</h2>
    <div className="milestone-scale" style={{ '--milestone-progress': `${Math.min(100, progress)}%` }}>
      {items.map((item, index) => (
        <div className={`milestone-point ${item.status === 'approved' ? 'done' : ''}`} key={item.id || index}>
          <span>{index + 1}</span>
          <small>{item.title}</small>
        </div>
      ))}
    </div>
  </section>;
}

function FileLinks({ files }) {
  const items = asArray(files);
  if (!items.length) return <p className="muted">Файлы не приложены</p>;
  return <div className="inline">
    {items.map((file, index) => <a className="button-link secondary" key={`${file.file_url}-${index}`} href={file.file_url} target="_blank" rel="noreferrer">{mediaLabel(file.file_type)}</a>)}
  </div>;
}

function mediaLabel(type) {
  return ({ image: 'Изображение', video: 'Видео', document: 'Документ' })[type] || 'Файл';
}

function money(value) {
  return new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', maximumFractionDigits: 0 }).format(value || 0);
}

function asArray(value) {
  return Array.isArray(value) ? value : [];
}

createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <AuthProvider>
      <Router>
        <Layout />
      </Router>
    </AuthProvider>
  </React.StrictMode>
);
