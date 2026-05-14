import React, { createContext, useContext, useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { Link, Navigate, Route, BrowserRouter as Router, Routes, useNavigate, useParams } from 'react-router-dom';
import './styles.css';

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';
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
          <Link to="/admin/projects">Админ</Link>
          <Link to="/me">Профиль</Link>
          {auth.user ? <button onClick={auth.logout}>Выйти</button> : <Link to="/login">Войти</Link>}
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
          <Route path="/projects/:id/edit" element={<ProjectForm edit />} />
          <Route path="/projects/:id/milestones" element={<Milestones />} />
          <Route path="/projects/:id/rewards" element={<Rewards />} />
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
  const [campaignType, setCampaignType] = useState('');
  useEffect(() => {
    api(`/projects${campaignType ? `?campaign_type=${campaignType}` : ''}`).then(data => setProjects(data.items || []));
  }, [campaignType]);
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
  const progress = Math.min(100, Math.round((project.funds.total_collected / project.goal_amount) * 100));
  return (
    <Link to={`/projects/${project.id}`} className="card project-card">
      <span className="pill">{project.campaign_type}</span>
      <h2>{project.title}</h2>
      <p>{project.short_description}</p>
      <div className="progress"><span style={{ width: `${progress}%` }} /></div>
      <strong>{money(project.funds.total_collected)} из {money(project.goal_amount)}</strong>
    </Link>
  );
}

function ProjectDetails() {
  const { id } = useParams();
  const auth = useAuth();
  const [data, setData] = useState(null);
  const [pledge, setPledge] = useState({ amount: 1000, reward_id: null });
  const [paymentId, setPaymentId] = useState('');
  const [message, setMessage] = useState('');
  const load = () => api(`/projects/${id}`).then(setData);
  useEffect(load, [id]);
  if (!data) return <p>Загрузка...</p>;
  const p = data.project;
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
  async function capturePayment(e) {
    e.preventDefault();
    try {
      const res = await api(`/payments/${paymentId}/mock-capture`, { method: 'POST', token: auth.token, body: {} });
      setMessage(`Платёж обработан: ${res.status}`);
      load();
    } catch (err) {
      setMessage(err.message);
    }
  }
  return (
    <section className="stack">
      <article className="card">
        <span className="pill">{p.status} · {p.campaign_type}</span>
        <h1>{p.title}</h1>
        <p className="lead">{p.short_description}</p>
        <p>{p.description}</p>
        <div className="inline">
          <Link className="button-link" to={`/projects/${id}/milestones`}>Этапы</Link>
          <Link className="button-link" to={`/projects/${id}/rewards`}>Вознаграждения</Link>
          <Link className="button-link" to={`/projects/${id}/updates`}>Обновления</Link>
          {isAuthor && <Link className="button-link" to={`/projects/${id}/edit`}>Редактировать</Link>}
          {isAuthor && (p.status === 'draft' || p.status === 'rejected') && <button onClick={submitForReview}>На модерацию</button>}
        </div>
        <div className="stats">
          <Metric label="Цель" value={money(p.goal_amount)} />
          <Metric label="Собрано" value={money(p.funds.total_collected)} />
          <Metric label="Зарезервировано" value={money(p.funds.total_reserved)} />
          <Metric label="Доступно" value={money(p.funds.total_available)} />
          <Metric label="Возвращено" value={money(p.funds.total_refunded)} />
        </div>
      </article>
      <div className="two-col">
        <Panel title="Этапы" items={data.milestones} render={m => `${m.position}. ${m.title} · ${money(m.amount_limit)} · ${m.status}`} />
        <Panel title="Вознаграждения" items={data.rewards} render={r => `${r.title} от ${money(r.min_amount)}`} />
      </div>
      <section className="card">
        <h2>Поддержать проект</h2>
        <form className="inline" onSubmit={support}>
          <input type="number" value={pledge.amount} onChange={e => setPledge({ ...pledge, amount: e.target.value })} />
          <select value={pledge.reward_id || ''} onChange={e => setPledge({ ...pledge, reward_id: e.target.value || null })}>
            <option value="">Без вознаграждения</option>
            {data.rewards.map(r => <option key={r.id} value={r.id}>{r.title}</option>)}
          </select>
          <button disabled={!auth.token}>Поддержать</button>
        </form>
        {message && <p className="notice">{message}</p>}
      </section>
      {isAdmin && <section className="card">
        <h2>Mock-платёж</h2>
        <form className="inline" onSubmit={capturePayment}>
          <input placeholder="Payment ID" value={paymentId} onChange={e => setPaymentId(e.target.value)} />
          <button disabled={!paymentId}>Подтвердить оплату</button>
        </form>
      </section>}
      <Panel title="Обновления" items={data.updates} render={u => `${u.title}: ${u.content}`} />
    </section>
  );
}

function ProjectForm({ edit }) {
  const { id } = useParams();
  const auth = useAuth();
  const navigate = useNavigate();
  const [categories, setCategories] = useState([]);
  const [form, setForm] = useState({ title: '', short_description: '', description: '', category_id: 1, campaign_type: 'reward', currency: 'RUB', goal_amount: 100000 });
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
      navigate(`/projects/${data.id}`);
    } catch (err) {
      setError(err.message);
    }
  }
  return (
    <section className="card">
      <h1>{edit ? 'Редактировать проект' : 'Новый проект'}</h1>
      <form onSubmit={submit} className="stack">
        <input placeholder="Название" value={form.title} onChange={e => setForm({ ...form, title: e.target.value })} />
        <input placeholder="Краткое описание" value={form.short_description} onChange={e => setForm({ ...form, short_description: e.target.value })} />
        <textarea placeholder="Полное описание" value={form.description} onChange={e => setForm({ ...form, description: e.target.value })} />
        <select value={form.category_id} onChange={e => setForm({ ...form, category_id: e.target.value })}>{categories.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}</select>
        <select value={form.campaign_type} onChange={e => setForm({ ...form, campaign_type: e.target.value })}><option value="reward">Reward</option><option value="donation">Donation</option></select>
        <input value={form.currency} onChange={e => setForm({ ...form, currency: e.target.value.toUpperCase() })} />
        <input type="number" value={form.goal_amount} onChange={e => setForm({ ...form, goal_amount: e.target.value })} />
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
  const [form, setForm] = useState({ title: '', content: '' });
  const [message, setMessage] = useState('');
  async function submit(e) {
    e.preventDefault();
    await api(`/projects/${id}/updates`, { method: 'POST', token: auth.token, body: form });
    setMessage('Обновление опубликовано');
  }
  return <FormCard title="Опубликовать обновление" onSubmit={submit}>
    <input placeholder="Заголовок" value={form.title} onChange={e => setForm({ ...form, title: e.target.value })} />
    <textarea placeholder="Контент" value={form.content} onChange={e => setForm({ ...form, content: e.target.value })} />
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
    <Panel title="Отчёты по этапам" items={items} render={item => <span>{item.project_title} · {item.milestone_title} · {item.report_text} <button onClick={() => review(item, 'approved')}>Одобрить</button> <button onClick={() => review(item, 'rejected')}>Отклонить</button></span>} />
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

function money(value) {
  return new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', maximumFractionDigits: 0 }).format(value || 0);
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
