import { StrictMode, useEffect, useState, type ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import {
  BarChart3,
  BookOpen,
  ChevronLeft,
  ChevronRight,
  Check,
  Clock3,
  Database,
  Edit3,
  Eye,
  EyeOff,
  Gauge,
  History,
  Keyboard,
  ListChecks,
  Languages,
  Loader2,
  LogOut,
  Moon,
  PanelLeftClose,
  PanelLeftOpen,
  Play,
  RotateCcw,
  Save,
  Search,
  Settings,
  SkipForward,
  Sparkles,
  Sun,
  TrendingUp,
  Upload,
  User,
} from 'lucide-react'
import {
  api,
  type AuthPayload,
  type Block,
  type Concept,
  type ConceptContent,
  type ConceptRow,
  type StatBucket,
  type Unit,
} from './api'
import './styles.css'

type View = 'dashboard' | 'browse' | 'quick' | 'review' | 'data' | 'profile'
type Lang = 'en' | 'zh'
type Theme = 'light' | 'dark'

type DashboardSummary = { totalConcepts: number; readyConcepts: number }
type DashboardProgress = {
  reviewedConcepts: number
  ratedConcepts: number
  weakConcepts: number
  averageMastery: number
  todayReviews: number
  todayMasteryGain: number
  shortTermReviews: number
  streakDays: number
}
type DashboardTrends = { daily: StatBucket[]; hourly: StatBucket[] }
type WeakArea = { label: string; weak: number; averageMastery: number }
type DashboardAlerts = { recent: unknown[]; weakConcepts: Concept[]; weakUnits: WeakArea[]; weakTopics: WeakArea[] }

type ImportStatus = {
  units: number
  topics: number
  concepts: number
  readyConcepts: number
  byUnit: { unitId: string; unit: string; concepts: number; ready: number }[]
  runs: { id: string; source: string; status: string; message: string; counts: string; createdAt: string }[]
}

const copy = {
  en: {
    today: 'Today',
    browse: 'Browse',
    quick: 'Quick rate',
    review: 'Review',
    data: 'Data',
    profile: 'Profile',
    logout: 'Log out',
    focus: 'Focus',
    theme: 'Theme',
    dark: 'Dark',
    light: 'Light',
    settings: 'Settings',
    userCenter: 'User center',
    cockpit: 'Your sprint cockpit',
    quickRate: 'Quick rate',
    createReview: 'Create review',
    totalTerms: 'Total terms',
    enrichedTerms: 'Study content',
    ratedTerms: 'Rated terms',
    weakTerms: 'Needs practice',
    todayReviews: 'Today',
    todayGain: 'Mastery gain',
    shortTerm: 'Short-term queue',
    streak: 'Streak',
    averageMastery: 'Average mastery',
    coverage: 'Coverage',
    countdown: 'AP exam countdown',
    countdownSmall: 'Target: 2026-05-12 12:00 UTC+8',
    dailyReviews: 'Daily reviews',
    last24h: 'Last 24h',
    weakUnits: 'Weak units',
    weakTopics: 'Weak topics',
    noWeakAreas: 'No weak rated areas yet.',
    cumulativeLearned: 'Cumulative learned',
    reminders: 'Reminders',
    noReminders: 'No weak rated terms yet.',
    reviewWeak: 'Review weak terms',
    browseTitle: 'Concept map',
    showCard: 'Show card',
    hideCard: 'Hide card',
    searchTerms: 'Search terms',
    selectConcept: 'Select a concept to inspect its card back.',
    allConcepts: 'All concepts',
    quickTitle: 'Calibrate mastery fast',
    startStack: 'Start stack',
    stackComplete: 'Stack complete',
    setupAnother: 'Set up another stack',
    jump: 'Jump',
    jumpPlaceholder: 'Go to #',
    skipRated: 'Skip rated',
    allItems: 'All',
    unrated: 'Unrated',
    rated: 'Rated',
    weakOnly: 'Weak',
    firstUnrated: 'First unrated',
    firstRated: 'First rated',
    previous: 'Previous',
    next: 'Next',
    skip: 'Skip',
    keyQuick: '0-5 rate / 7 previous / 8 next',
    reviewTitle: 'Create a review set',
    wordsPerSession: 'Words per session',
    random: 'Random',
    outlineOrder: 'Outline order',
    startReview: 'Start review',
    sessionSummary: 'Session summary',
    newSet: 'New set',
    reviewed: 'Reviewed',
    know: 'Know',
    fuzzy: 'Fuzzy',
    stillWeak: 'Still weak',
    dontKnow: "Don't know",
    understandNow: 'I understand now',
    undo: 'Undo',
    dataTitle: 'Concept data manager',
    reimport: 'Re-import',
    ready: 'Ready',
    searchData: 'Search concept data',
    pickEdit: 'Pick a concept to edit definitions, examples, pitfalls, and notes.',
    definition: 'Definition',
    examples: 'Examples',
    pitfalls: 'Pitfalls',
    notes: 'Notes',
    saveContent: 'Save content',
    saved: 'Saved',
    noContent: 'No enriched content yet. The term is still available for recognition review.',
    source: 'Source',
    profileTitle: 'Workspace controls',
    name: 'Name',
    tenant: 'Workspace',
    saveProfile: 'Save profile',
    system: 'System',
    avatar: 'Avatar',
    uploadAvatar: 'Upload avatar',
    unit: 'Unit',
    topic: 'Topic',
    allUnits: 'All units',
    allTopics: 'All topics in scope',
    loading: 'Loading',
    authTitle: 'Final Sprint',
    authCopy: 'Rate every concept, review weak terms, and keep the whole knowledge map in one quiet workspace.',
    register: 'Register',
    login: 'Login',
    email: 'Email',
    password: 'Password',
    inviteCode: 'Class code',
    createWorkspace: 'Create workspace',
    enterWorkspace: 'Enter workspace',
  },
  zh: {
    today: '首页',
    browse: '浏览',
    quick: '快速打分',
    review: '复习',
    data: '数据',
    profile: '用户',
    logout: '退出',
    focus: '专注',
    theme: '主题',
    dark: '夜间',
    light: '日间',
    settings: '设置',
    userCenter: '用户中心',
    cockpit: '冲刺仪表盘',
    quickRate: '快速打分',
    createReview: '开始复习',
    totalTerms: '总词条',
    enrichedTerms: '可学内容',
    ratedTerms: '已打分',
    weakTerms: '需加强',
    todayReviews: '今日复习',
    todayGain: '今日增长',
    shortTerm: '短期复习',
    streak: '连续天数',
    averageMastery: '平均掌握度',
    coverage: '覆盖情况',
    countdown: 'AP 考试倒计时',
    countdownSmall: '目标：2026-05-12 12:00 UTC+8',
    dailyReviews: '每日复习',
    last24h: '过去 24 小时',
    weakUnits: '薄弱单元',
    weakTopics: '薄弱主题',
    noWeakAreas: '目前没有已标记的薄弱区域。',
    cumulativeLearned: '累计掌握',
    reminders: '提醒',
    noReminders: '目前还没有已标记的不熟词汇。',
    reviewWeak: '复习薄弱词',
    browseTitle: '概念地图',
    showCard: '显示卡片',
    hideCard: '隐藏卡片',
    searchTerms: '搜索词条',
    selectConcept: '选择一个概念查看卡片背面。',
    allConcepts: '全部概念',
    quickTitle: '快速校准掌握度',
    startStack: '开始',
    stackComplete: '已完成',
    setupAnother: '重新设置',
    jump: '跳转',
    jumpPlaceholder: '跳到序号',
    skipRated: '跳过已打分',
    allItems: '全部',
    unrated: '未打分',
    rated: '已打分',
    weakOnly: '薄弱',
    firstUnrated: '第一个未打分',
    firstRated: '第一个已打分',
    previous: '上一个',
    next: '下一个',
    skip: '跳过',
    keyQuick: '0-5 打分 / 7 上一个 / 8 下一个',
    reviewTitle: '创建复习组',
    wordsPerSession: '每组数量',
    random: '随机',
    outlineOrder: '按大纲',
    startReview: '开始复习',
    sessionSummary: '复习总结',
    newSet: '新建一组',
    reviewed: '已复习',
    know: '会',
    fuzzy: '模糊',
    stillWeak: '仍薄弱',
    dontKnow: '不会',
    understandNow: '现在理解了',
    undo: '撤销',
    dataTitle: '概念数据管理',
    reimport: '重新导入',
    ready: '已就绪',
    searchData: '搜索概念数据',
    pickEdit: '选择一个概念编辑定义、例子、误区和笔记。',
    definition: '定义',
    examples: '例子',
    pitfalls: '易错点',
    notes: '笔记',
    saveContent: '保存内容',
    saved: '已保存',
    noContent: '还没有补充内容。仍可用于识别式复习。',
    source: '来源',
    profileTitle: '工作区控制',
    name: '姓名',
    tenant: '工作区',
    saveProfile: '保存资料',
    system: '系统',
    avatar: '头像',
    uploadAvatar: '上传头像',
    unit: '单元',
    topic: '主题',
    allUnits: '全部单元',
    allTopics: '当前范围全部主题',
    loading: '加载中',
    authTitle: '期末冲刺',
    authCopy: '给每个概念打分，复习薄弱词，把整张知识地图放进一个安静的工作台。',
    register: '注册',
    login: '登录',
    email: '邮箱',
    password: '密码',
    inviteCode: '班级码',
    createWorkspace: '创建工作区',
    enterWorkspace: '进入工作区',
  },
}

function App() {
  const [auth, setAuth] = useState<AuthPayload | null>(null)
  const [view, setView] = useState<View>('dashboard')
  const [collapsed, setCollapsed] = useState(false)
  const [theme, setTheme] = useState<Theme>((localStorage.getItem('apPsychTheme') as Theme) || 'light')
  const [lang, setLang] = useState<Lang>((localStorage.getItem('apPsychLang') as Lang) || 'en')
  const [userMenuOpen, setUserMenuOpen] = useState(false)
  const t = copy[lang]
  const isAdmin = auth?.user.role === 'admin'

  useEffect(() => {
    document.documentElement.dataset.theme = theme
    localStorage.setItem('apPsychTheme', theme)
  }, [theme])

  useEffect(() => {
    localStorage.setItem('apPsychLang', lang)
  }, [lang])

  useEffect(() => {
    if (view === 'data' && auth && !isAdmin) setView('dashboard')
  }, [view, auth, isAdmin])

  useEffect(() => {
    if (!api.token) return
    api
      .request<Omit<AuthPayload, 'token'>>('/api/me')
      .then((payload) => setAuth({ ...payload, token: api.token }))
      .catch(() => api.logout())
  }, [])

  if (!auth) return <AuthScreen onAuth={setAuth} t={t} />

  return (
    <div className={`app-shell ${collapsed ? 'nav-collapsed' : ''}`}>
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark">AP</div>
          {!collapsed && (
            <div>
              <strong>Psych Sprint</strong>
              <span>{auth.tenant.name}</span>
            </div>
          )}
        </div>
        <button className="icon-line" onClick={() => setCollapsed((value) => !value)} title="Collapse sidebar">
          {collapsed ? <PanelLeftOpen size={17} /> : <PanelLeftClose size={17} />}
          {!collapsed && t.focus}
        </button>
        <nav>
          <NavButton active={view === 'dashboard'} collapsed={collapsed} icon={<Gauge />} label={t.today} onClick={() => setView('dashboard')} />
          <NavButton active={view === 'browse'} collapsed={collapsed} icon={<BookOpen />} label={t.browse} onClick={() => setView('browse')} />
          <NavButton active={view === 'quick'} collapsed={collapsed} icon={<Keyboard />} label={t.quick} onClick={() => setView('quick')} />
          <NavButton active={view === 'review'} collapsed={collapsed} icon={<Eye />} label={t.review} onClick={() => setView('review')} />
          {isAdmin && <NavButton active={view === 'data'} collapsed={collapsed} icon={<Database />} label={t.data} onClick={() => setView('data')} />}
        </nav>
        <div className="sidebar-tools">
          <button className="icon-line" onClick={() => setTheme(theme === 'light' ? 'dark' : 'light')} title="Theme">
            {theme === 'light' ? <Moon size={16} /> : <Sun size={16} />}
            {!collapsed && (theme === 'light' ? t.dark : t.light)}
          </button>
          <button className="icon-line" onClick={() => setLang(lang === 'en' ? 'zh' : 'en')} title="Language">
            <Languages size={16} />
            {!collapsed && (lang === 'en' ? '中文' : 'English')}
          </button>
          <div className="user-dock">
            <button className="user-chip" onClick={() => setUserMenuOpen((value) => !value)} title={t.userCenter}>
              <Avatar user={auth.user} />
              {!collapsed && <span><strong>{auth.user.name}</strong><small>{auth.user.role}</small></span>}
            </button>
            {userMenuOpen && (
              <div className="user-menu">
                <button onClick={() => { setView('profile'); setUserMenuOpen(false) }}><Settings size={16} /> {t.settings}</button>
                <button
                  onClick={() => {
                    api.logout()
                    setAuth(null)
                  }}
                >
                  <LogOut size={16} /> {t.logout}
                </button>
              </div>
            )}
          </div>
        </div>
      </aside>
      <main className="workspace">
        <div className="view-frame" key={view}>
          {view === 'dashboard' && <DashboardView t={t} goReview={() => setView('review')} goQuick={() => setView('quick')} />}
          {view === 'browse' && <BrowseView t={t} />}
          {view === 'quick' && <QuickRateView t={t} />}
          {view === 'review' && <ReviewView t={t} />}
          {view === 'data' && isAdmin && <DataView t={t} />}
          {view === 'profile' && (
            <ProfileView
              t={t}
              auth={auth}
              onAuth={setAuth}
              theme={theme}
              setTheme={setTheme}
              lang={lang}
              setLang={setLang}
            />
          )}
        </div>
      </main>
    </div>
  )
}

function AuthScreen({ onAuth, t }: { onAuth: (payload: AuthPayload) => void; t: typeof copy.en }) {
  const [mode, setMode] = useState<'login' | 'register'>('register')
  const [tenantName, setTenantName] = useState('')
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [inviteCode, setInviteCode] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit() {
    setBusy(true)
    setError('')
    try {
      const payload =
        mode === 'register'
          ? await api.register({ tenantName, name, email, password, inviteCode })
          : await api.login({ email, password })
      api.setToken(payload.token)
      onAuth(payload)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Authentication failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="auth-page">
      <section className="auth-panel">
        <div className="auth-copy">
          <div className="badge">AP Psychology</div>
          <h1>{t.authTitle}</h1>
          <p>{t.authCopy}</p>
        </div>
        <div className="auth-card">
          <div className="segmented">
            <button className={mode === 'register' ? 'active' : ''} onClick={() => setMode('register')}>{t.register}</button>
            <button className={mode === 'login' ? 'active' : ''} onClick={() => setMode('login')}>{t.login}</button>
          </div>
          {mode === 'register' && (
            <>
              <label>{t.tenant}<input placeholder="AP Study Room" value={tenantName} onChange={(e) => setTenantName(e.target.value)} /></label>
              <label>{t.name}<input placeholder="Student" value={name} onChange={(e) => setName(e.target.value)} /></label>
              <label>{t.inviteCode}<input value={inviteCode} onChange={(e) => setInviteCode(e.target.value)} /></label>
            </>
          )}
          <label>{t.email}<input placeholder="student@example.com" value={email} onChange={(e) => setEmail(e.target.value)} /></label>
          <label>{t.password}<input type="password" value={password} onChange={(e) => setPassword(e.target.value)} /></label>
          {error && <div className="error">{error}</div>}
          <button className="primary" disabled={busy} onClick={submit}>{busy ? <Loader2 className="spin" size={16} /> : null}{mode === 'register' ? t.createWorkspace : t.enterWorkspace}</button>
        </div>
      </section>
    </main>
  )
}

function DashboardView({ t, goReview, goQuick }: { t: typeof copy.en; goReview: () => void; goQuick: () => void }) {
  const [summary, setSummary] = useState<DashboardSummary | null>(null)
  const [progress, setProgress] = useState<DashboardProgress | null>(null)
  const [trends, setTrends] = useState<DashboardTrends | null>(null)
  const [alerts, setAlerts] = useState<DashboardAlerts | null>(null)
  const [now, setNow] = useState(Date.now())
  useEffect(() => {
    void api.request<DashboardSummary>('/api/dashboard/summary').then(setSummary)
    void api.request<DashboardProgress>('/api/dashboard/progress').then(setProgress)
    void api.request<DashboardTrends>('/api/dashboard/trends').then(setTrends)
    void api.request<DashboardAlerts>('/api/dashboard/alerts').then(setAlerts)
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [])

  const countdown = examCountdown(now)
  const cumulative = cumulativeLearned(trends?.daily ?? [])
  const readyPct = percent(summary?.readyConcepts ?? 0, summary?.totalConcepts ?? 0)
  const ratedPct = percent(progress?.ratedConcepts ?? 0, summary?.totalConcepts ?? 0)

  return (
    <section className="page">
      <Header
        eyebrow={t.today}
        title={t.cockpit}
        action={<div className="action-row"><button className="secondary" onClick={goQuick}><Keyboard size={16} /> {t.quickRate}</button><button className="primary" onClick={goReview}><Sparkles size={16} /> {t.createReview}</button></div>}
      />
      <div className="metrics">
        <Metric label={t.todayReviews} value={progress?.todayReviews ?? 0} loading={!progress} />
        <Metric label={t.todayGain} value={progress?.todayMasteryGain ?? 0} loading={!progress} />
        <Metric label={t.shortTerm} value={progress?.shortTermReviews ?? 0} loading={!progress} />
        <Metric label={t.streak} value={progress?.streakDays ?? 0} loading={!progress} />
      </div>
      <div className="coverage-strip">
        <span>{t.coverage}</span>
        <strong>{t.totalTerms}: {summary?.totalConcepts ?? 0}</strong>
        <strong>{t.enrichedTerms}: {summary ? `${summary.readyConcepts} (${readyPct}%)` : '...'}</strong>
        <strong>{t.ratedTerms}: {progress && summary ? `${progress.ratedConcepts} (${ratedPct}%)` : '...'}</strong>
        <strong>{t.weakTerms}: {progress?.weakConcepts ?? 0}</strong>
      </div>
      <div className="dashboard-grid">
        <div className="progress-band">
          <div>
            <span>{t.averageMastery}</span>
            <strong>{(progress?.averageMastery ?? 0).toFixed(2)} / 5</strong>
          </div>
          <div className="mastery-bar"><i style={{ width: `${((progress?.averageMastery ?? 0) / 5) * 100}%` }} /></div>
        </div>
        <div className="countdown-card">
          <span>{t.countdown}</span>
          <strong>{countdown.days}d {countdown.hours}h {countdown.minutes}m {countdown.seconds}s</strong>
          <small>{t.countdownSmall}</small>
        </div>
      </div>
      <div className="reminder-card">
        <div>
          <span>{t.reminders}</span>
          <strong>{alerts?.weakConcepts?.[0]?.term ?? t.noReminders}</strong>
        </div>
        <button className="secondary" onClick={goReview}>{t.reviewWeak}</button>
      </div>
      <div className="chart-grid three">
        <ChartCard title={t.dailyReviews} icon={<BarChart3 size={17} />} data={trends?.daily ?? []} mode="reviews" />
        <ChartCard title={t.last24h} icon={<TrendingUp size={17} />} data={trends?.hourly ?? []} mode="gain" />
        <ChartCard title={t.cumulativeLearned} icon={<History size={17} />} data={cumulative} mode="learned" />
      </div>
      <div className="insight-grid">
        <WeakAreaCard title={t.weakUnits} icon={<ListChecks size={17} />} rows={alerts?.weakUnits ?? []} empty={t.noWeakAreas} />
        <WeakAreaCard title={t.weakTopics} icon={<Clock3 size={17} />} rows={alerts?.weakTopics ?? []} empty={t.noWeakAreas} />
      </div>
    </section>
  )
}

function BrowseView({ t }: { t: typeof copy.en }) {
  const [units, setUnits] = useState<Unit[]>([])
  const [concepts, setConcepts] = useState<ConceptRow[]>([])
  const [selectedTopic, setSelectedTopic] = useState('')
  const [search, setSearch] = useState('')
  const [active, setActive] = useState<ConceptRow | null>(null)
  const [loading, setLoading] = useState(true)
  const [panelHidden, setPanelHidden] = useState(false)

  useEffect(() => {
    api.request<Unit[]>('/api/units').then(setUnits)
  }, [])
  useEffect(() => {
    const params = new URLSearchParams()
    if (selectedTopic) params.set('topicId', selectedTopic)
    if (search) params.set('search', search)
    setLoading(true)
    api.request<ConceptRow[]>(`/api/concepts?${params.toString()}`).then(setConcepts).finally(() => setLoading(false))
  }, [selectedTopic, search])

  async function rate(concept: ConceptRow, rating: number) {
    const state = await api.request<ConceptRow['state']>(`/api/concepts/${concept.id}/rating`, {
      method: 'PATCH',
      body: JSON.stringify({ rating }),
    })
    const updated = { ...concept, state }
    setConcepts((rows) => rows.map((row) => (row.id === concept.id ? updated : row)))
    if (active?.id === concept.id) setActive(updated)
  }

  return (
    <section className="page">
      <Header
        eyebrow={t.browse}
        title={t.browseTitle}
        action={<button className="secondary" onClick={() => setPanelHidden((value) => !value)}>{panelHidden ? <Eye size={16} /> : <EyeOff size={16} />} {panelHidden ? t.showCard : t.hideCard}</button>}
      />
      <div className={`browse-layout ${panelHidden ? 'panel-off' : ''}`}>
        <TopicList t={t} units={units} selectedTopic={selectedTopic} setSelectedTopic={setSelectedTopic} />
        <div className="concept-list">
          <label className="search"><Search size={16} /><input placeholder={t.searchTerms} value={search} onChange={(e) => setSearch(e.target.value)} /></label>
          {loading ? <ListSkeleton /> : (
            <div className="table">
              {concepts.slice(0, 180).map((concept) => (
                <button className="concept-row" key={concept.id} onClick={() => setActive(concept)}>
                  <span>
                    <strong>{concept.term}</strong>
                    <small>{concept.topic?.title}</small>
                  </span>
                  <Rating value={concept.state.mastery} onRate={(rating) => rate(concept, rating)} />
                </button>
              ))}
            </div>
          )}
        </div>
        {!panelHidden && <ConceptPanel t={t} concept={active} onRate={(rating) => active && rate(active, rating)} />}
      </div>
    </section>
  )
}

function QuickRateView({ t }: { t: typeof copy.en }) {
  const [units, setUnits] = useState<Unit[]>([])
  const [unitId, setUnitId] = useState('')
  const [topicId, setTopicId] = useState('')
  const [queue, setQueue] = useState<ConceptRow[]>([])
  const [index, setIndex] = useState(0)
  const [jump, setJump] = useState('')
  const [progressFilter, setProgressFilter] = useState<'all' | 'zero' | 'nonzero' | 'weak'>('zero')
  const [skipRated, setSkipRated] = useState(true)
  const [loading, setLoading] = useState(false)
  const [started, setStarted] = useState(false)
  const current = queue[index]

  useEffect(() => {
    api.request<Unit[]>('/api/units').then(setUnits)
  }, [])
  useEffect(() => {
    function handleKey(event: KeyboardEvent) {
      if (!started || !current) return
      if (/^[0-5]$/.test(event.key)) {
        void rate(Number(event.key))
      }
      if (event.key === '7') setIndex((value) => Math.max(0, value - 1))
      if (event.key === '8') setIndex((value) => Math.min(queue.length - 1, value + 1))
    }
    window.addEventListener('keydown', handleKey)
    return () => window.removeEventListener('keydown', handleKey)
  }, [started, current, queue.length])

  async function start() {
    const params = new URLSearchParams()
    if (topicId) params.set('topicId', topicId)
    else if (unitId) params.set('unitId', unitId)
    const filter = skipRated ? 'zero' : progressFilter
    if (filter !== 'all') params.set('progress', filter)
    setLoading(true)
    const rows = await api.request<ConceptRow[]>(`/api/concepts?${params.toString()}`)
    setQueue(rows)
    setIndex(0)
    setStarted(true)
    setLoading(false)
  }

  async function rate(value: number) {
    if (!current) return
    const state = await api.request<ConceptRow['state']>(`/api/concepts/${current.id}/rating`, {
      method: 'PATCH',
      body: JSON.stringify({ rating: value }),
    })
    setQueue((rows) => rows.map((row) => (row.id === current.id ? { ...row, state } : row)))
    setIndex((value) => value + 1)
  }

  function go(delta: number) {
    setIndex((value) => Math.max(0, Math.min(queue.length - 1, value + delta)))
  }

  function jumpTo(value: string) {
    const next = Number(value)
    if (!Number.isFinite(next)) return
    setIndex(Math.max(0, Math.min(queue.length - 1, next - 1)))
  }

  function jumpToStatus(kind: 'zero' | 'nonzero') {
    const found = queue.findIndex((row) => kind === 'zero' ? row.state.mastery === 0 : row.state.mastery > 0)
    if (found >= 0) setIndex(found)
  }

  if (!started) {
    return (
      <section className="page">
        <Header eyebrow={t.quickRate} title={t.quickTitle} />
        <SetupCard>
          <ScopePicker t={t} units={units} unitId={unitId} topicId={topicId} setUnitId={setUnitId} setTopicId={setTopicId} />
          <label className="check-line"><input type="checkbox" checked={skipRated} onChange={(e) => setSkipRated(e.target.checked)} /> {t.skipRated}</label>
          <div className="segmented four">
            <button className={progressFilter === 'all' ? 'active' : ''} onClick={() => { setProgressFilter('all'); setSkipRated(false) }}>{t.allItems}</button>
            <button className={progressFilter === 'zero' ? 'active' : ''} onClick={() => { setProgressFilter('zero'); setSkipRated(true) }}>{t.unrated}</button>
            <button className={progressFilter === 'nonzero' ? 'active' : ''} onClick={() => { setProgressFilter('nonzero'); setSkipRated(false) }}>{t.rated}</button>
            <button className={progressFilter === 'weak' ? 'active' : ''} onClick={() => { setProgressFilter('weak'); setSkipRated(false) }}>{t.weakOnly}</button>
          </div>
          <button className="primary" onClick={start} disabled={loading}>{loading ? <Loader2 className="spin" size={16} /> : <Play size={16} />} {t.startStack}</button>
        </SetupCard>
      </section>
    )
  }

  if (!current) {
    return (
      <section className="page">
        <Header eyebrow={t.quickRate} title={t.stackComplete} />
        <div className="empty-state"><Check size={28} /><strong>{queue.length} {t.totalTerms.toLowerCase()}</strong><button className="secondary" onClick={() => setStarted(false)}>{t.setupAnother}</button></div>
      </section>
    )
  }

  return (
    <section className="page focus-page">
      <Header eyebrow={t.quickRate} title={`${index + 1} / ${queue.length}`} action={<KeyboardHint text={t.keyQuick} />} />
      <div className="quick-toolbar">
        <button className="secondary" onClick={() => go(-1)}><ChevronLeft size={16} /> {t.previous}</button>
        <button className="secondary" onClick={() => go(1)}><ChevronRight size={16} /> {t.next}</button>
        <button className="secondary" onClick={() => go(1)}><SkipForward size={16} /> {t.skip}</button>
        <button className="secondary" onClick={() => jumpToStatus('zero')}>{t.firstUnrated}</button>
        <button className="secondary" onClick={() => jumpToStatus('nonzero')}>{t.firstRated}</button>
        <label className="jump-box"><input placeholder={t.jumpPlaceholder} value={jump} onChange={(e) => setJump(e.target.value)} onKeyDown={(e) => { if (e.key === 'Enter') jumpTo(jump) }} /><button className="secondary" onClick={() => jumpTo(jump)}>{t.jump}</button></label>
      </div>
      <div className="quick-card">
        <small>{current.topic?.title}</small>
        <h2>{current.term}</h2>
        <Rating value={current.state.mastery} onRate={rate} />
      </div>
    </section>
  )
}

function ReviewView({ t }: { t: typeof copy.en }) {
  const [units, setUnits] = useState<Unit[]>([])
  const [unitId, setUnitId] = useState('')
  const [topicId, setTopicId] = useState('')
  const [limit, setLimit] = useState(30)
  const [order, setOrder] = useState<'random' | 'outline'>('random')
  const [queue, setQueue] = useState<Concept[]>([])
  const [index, setIndex] = useState(0)
  const [staged, setStaged] = useState<'know' | 'fuzzy' | 'unknown' | null>(null)
  const [loading, setLoading] = useState(false)
  const [started, setStarted] = useState(false)
  const [summary, setSummary] = useState<{ response: string; before: number; after: number }[]>([])
  const current = queue[index]

  useEffect(() => {
    api.request<Unit[]>('/api/units').then(setUnits)
  }, [])
  useEffect(() => {
    function handleKey(event: KeyboardEvent) {
      if (!started || !current) return
      const key = event.key.toLowerCase()
      if (!staged) {
        if (key === 'a') setStaged('know')
        if (key === 's') setStaged('fuzzy')
        if (key === 'd') setStaged('unknown')
        return
      }
      if (key === 'u') setStaged(null)
      if (staged === 'know' && (key === 'd' || event.key === 'Enter')) void commit('know')
      if (staged !== 'know' && (key === 'a' || event.key === 'Enter')) void commit('fuzzy')
      if (staged !== 'know' && key === 's') void commit('unknown')
    }
    window.addEventListener('keydown', handleKey)
    return () => window.removeEventListener('keydown', handleKey)
  }, [started, current, staged])

  async function start() {
    const params = new URLSearchParams({ limit: String(limit), order })
    if (topicId) params.set('topicId', topicId)
    else if (unitId) params.set('unitId', unitId)
    setLoading(true)
    const rows = await api.request<Concept[]>(`/api/review/next?${params.toString()}`)
    setQueue(rows)
    setIndex(0)
    setSummary([])
    setStaged(null)
    setStarted(true)
    setLoading(false)
  }

  async function commit(response: 'know' | 'fuzzy' | 'unknown') {
    if (!current) return
    const payload = await api.request<{ state: { mastery: number }; event: { masteryBefore: number; masteryAfter: number; response: string } }>('/api/review/events', {
      method: 'POST',
      body: JSON.stringify({ conceptId: current.id, cardId: current.cards?.[0]?.id ?? '', response }),
    })
    setSummary((rows) => [...rows, { response: payload.event.response, before: payload.event.masteryBefore, after: payload.event.masteryAfter }])
    setStaged(null)
    setIndex((value) => value + 1)
  }

  if (!started) {
    return (
      <section className="page">
        <Header eyebrow={t.review} title={t.reviewTitle} />
        <SetupCard>
          <ScopePicker t={t} units={units} unitId={unitId} topicId={topicId} setUnitId={setUnitId} setTopicId={setTopicId} />
          <label>{t.wordsPerSession}<input type="number" min={5} max={200} value={limit} onChange={(e) => setLimit(Number(e.target.value))} /></label>
          <div className="segmented inline">
            <button className={order === 'random' ? 'active' : ''} onClick={() => setOrder('random')}>{t.random}</button>
            <button className={order === 'outline' ? 'active' : ''} onClick={() => setOrder('outline')}>{t.outlineOrder}</button>
          </div>
          <button className="primary" onClick={start} disabled={loading}>{loading ? <Loader2 className="spin" size={16} /> : <Play size={16} />} {t.startReview}</button>
        </SetupCard>
      </section>
    )
  }

  if (!current) {
    return (
      <section className="page">
        <Header eyebrow={t.review} title={t.sessionSummary} action={<button className="secondary" onClick={() => setStarted(false)}><RotateCcw size={16} /> {t.newSet}</button>} />
        <div className="metrics">
          <Metric label={t.reviewed} value={summary.length} />
          <Metric label={t.know} value={summary.filter((row) => row.response === 'know').length} />
          <Metric label={t.fuzzy} value={summary.filter((row) => row.response === 'fuzzy').length} />
          <Metric label={t.stillWeak} value={summary.filter((row) => row.response === 'unknown').length} />
        </div>
      </section>
    )
  }

  const reviewHint = !staged
    ? `A ${t.know} / S ${t.fuzzy} / D ${t.dontKnow}`
    : staged === 'know'
      ? `D ${t.next} / U ${t.undo}`
      : `A ${t.understandNow} / S ${t.stillWeak} / U ${t.undo}`

  return (
    <section className="page focus-page">
      <Header eyebrow={t.review} title={`${index + 1} / ${queue.length}`} action={<KeyboardHint text={reviewHint} />} />
      <div className="review-card">
        <small>{current.topic?.title}</small>
        <h2>{current.term}</h2>
        {!staged ? (
          <div className="review-actions">
            <button className="success" onClick={() => setStaged('know')}>A {t.know}</button>
            <button className="secondary" onClick={() => setStaged('fuzzy')}>S {t.fuzzy}</button>
            <button className="danger" onClick={() => setStaged('unknown')}>D {t.dontKnow}</button>
          </div>
        ) : (
          <>
            <RichContent t={t} content={current.content} />
            <div className="review-actions">
              <button className="secondary" onClick={() => setStaged(null)}><RotateCcw size={16} /> {t.undo}</button>
              {staged === 'fuzzy' || staged === 'unknown' ? (
                <>
                  <button className="success" onClick={() => commit('fuzzy')}>A {t.understandNow}</button>
                  <button className="danger" onClick={() => commit('unknown')}>S {t.stillWeak}</button>
                </>
              ) : (
                <button className="primary" onClick={() => commit(staged)}>D {t.next}</button>
              )}
            </div>
          </>
        )}
      </div>
    </section>
  )
}

function DataView({ t }: { t: typeof copy.en }) {
  const [status, setStatus] = useState<ImportStatus | null>(null)
  const [concepts, setConcepts] = useState<ConceptRow[]>([])
  const [search, setSearch] = useState('')
  const [selected, setSelected] = useState<ConceptRow | null>(null)
  const [busy, setBusy] = useState(false)
  const [loading, setLoading] = useState(true)
  const loadStatus = () => api.request<ImportStatus>('/api/import/status').then(setStatus)
  const loadConcepts = () => {
    const params = new URLSearchParams()
    if (search) params.set('search', search)
    setLoading(true)
    return api.request<ConceptRow[]>(`/api/concepts?${params.toString()}`).then(setConcepts).finally(() => setLoading(false))
  }
  useEffect(() => { void loadStatus() }, [])
  useEffect(() => { void loadConcepts() }, [search])

  async function runImport() {
    setBusy(true)
    await api.request('/api/import/run', { method: 'POST', body: '{}' })
    await Promise.all([loadStatus(), loadConcepts()])
    setBusy(false)
  }

  return (
    <section className="page">
      <Header eyebrow={t.data} title={t.dataTitle} action={<button className="primary" disabled={busy} onClick={runImport}>{busy ? <Loader2 className="spin" size={16} /> : <Database size={16} />} {t.reimport}</button>} />
      <div className="data-layout">
        <div className="data-list">
          <div className="metrics compact">
            <Metric label={t.totalTerms} value={status?.concepts ?? 0} loading={!status} />
            <Metric label={t.ready} value={status?.readyConcepts ?? 0} loading={!status} />
          </div>
          <label className="search"><Search size={16} /><input placeholder={t.searchData} value={search} onChange={(e) => setSearch(e.target.value)} /></label>
          {loading ? <ListSkeleton /> : (
            <div className="table">
              {concepts.slice(0, 250).map((concept) => (
                <button className="concept-row" key={concept.id} onClick={() => setSelected(concept)}>
                  <span><strong>{concept.term}</strong><small>{sourceLabel(concept.content?.source ?? concept.contentStatus)}</small></span>
                  <Edit3 size={16} />
                </button>
              ))}
            </div>
          )}
        </div>
        <ConceptEditor t={t} concept={selected} onSaved={(concept) => setSelected(concept as ConceptRow)} />
      </div>
    </section>
  )
}

function ProfileView({ t, auth, onAuth, theme, setTheme, lang, setLang }: { t: typeof copy.en; auth: AuthPayload; onAuth: (payload: AuthPayload) => void; theme: Theme; setTheme: (theme: Theme) => void; lang: Lang; setLang: (lang: Lang) => void }) {
  const [name, setName] = useState(auth.user.name)
  const [tenantName, setTenantName] = useState(auth.tenant.name)
  const [avatarDataUrl, setAvatarDataUrl] = useState(auth.user.avatarDataUrl ?? '')
  const [saved, setSaved] = useState(false)
  async function save() {
    const payload = await api.request<Omit<AuthPayload, 'token'>>('/api/me', {
      method: 'PATCH',
      body: JSON.stringify({ name, tenantName, avatarDataUrl }),
    })
    onAuth({ ...payload, token: auth.token })
    setSaved(true)
    setTimeout(() => setSaved(false), 1200)
  }
  function loadAvatar(file?: File) {
    if (!file) return
    const reader = new FileReader()
    reader.onload = () => setAvatarDataUrl(String(reader.result ?? ''))
    reader.readAsDataURL(file)
  }
  return (
    <section className="page">
      <Header eyebrow={t.profile} title={t.profileTitle} />
      <div className="settings-grid">
        <div className="settings-card">
          <h3>{t.userCenter}</h3>
          <div className="avatar-upload">
            <Avatar user={{ ...auth.user, avatarDataUrl }} />
            <label className="secondary upload-button"><Upload size={16} /> {t.uploadAvatar}<input type="file" accept="image/*" onChange={(e) => loadAvatar(e.target.files?.[0])} /></label>
          </div>
          <label>{t.name}<input value={name} onChange={(e) => setName(e.target.value)} /></label>
          <label>{t.tenant}<input value={tenantName} onChange={(e) => setTenantName(e.target.value)} /></label>
          <button className="primary" onClick={save}><Save size={16} /> {saved ? t.saved : t.saveProfile}</button>
        </div>
        <div className="settings-card">
          <h3>{t.system}</h3>
          <div className="segmented inline">
            <button className={theme === 'light' ? 'active' : ''} onClick={() => setTheme('light')}>{t.light}</button>
            <button className={theme === 'dark' ? 'active' : ''} onClick={() => setTheme('dark')}>{t.dark}</button>
          </div>
          <div className="segmented inline">
            <button className={lang === 'en' ? 'active' : ''} onClick={() => setLang('en')}>English</button>
            <button className={lang === 'zh' ? 'active' : ''} onClick={() => setLang('zh')}>中文</button>
          </div>
        </div>
      </div>
    </section>
  )
}

function TopicList({ t, units, selectedTopic, setSelectedTopic }: { t: typeof copy.en; units: Unit[]; selectedTopic: string; setSelectedTopic: (id: string) => void }) {
  return (
    <aside className="topic-list">
      <button className={selectedTopic === '' ? 'active' : ''} onClick={() => setSelectedTopic('')}>{t.allConcepts}</button>
      {units.map((unit) => (
        <div key={unit.id}>
          <h3>{unit.title}</h3>
          {unit.topics.map((topic) => (
            <button key={topic.id} className={selectedTopic === topic.id ? 'active' : ''} onClick={() => setSelectedTopic(topic.id)}>
              {topic.title}
            </button>
          ))}
        </div>
      ))}
    </aside>
  )
}

function ScopePicker({ t, units, unitId, topicId, setUnitId, setTopicId }: { t: typeof copy.en; units: Unit[]; unitId: string; topicId: string; setUnitId: (id: string) => void; setTopicId: (id: string) => void }) {
  const topics = units.find((unit) => unit.id === unitId)?.topics ?? []
  return (
    <div className="scope-grid">
      <label>{t.unit}
        <select value={unitId} onChange={(e) => { setUnitId(e.target.value); setTopicId('') }}>
          <option value="">{t.allUnits}</option>
          {units.map((unit) => <option key={unit.id} value={unit.id}>{unit.title}</option>)}
        </select>
      </label>
      <label>{t.topic}
        <select value={topicId} onChange={(e) => setTopicId(e.target.value)} disabled={!unitId}>
          <option value="">{t.allTopics}</option>
          {topics.map((topic) => <option key={topic.id} value={topic.id}>{topic.title}</option>)}
        </select>
      </label>
    </div>
  )
}

function ConceptPanel({ t, concept, onRate }: { t: typeof copy.en; concept: ConceptRow | null; onRate: (rating: number) => void }) {
  if (!concept) return <aside className="concept-panel empty">{t.selectConcept}</aside>
  return (
    <aside className="concept-panel">
      <small>{concept.unit?.title}</small>
      <h2>{concept.term}</h2>
      <Rating value={concept.state.mastery} onRate={onRate} />
      <RichContent t={t} content={concept.content} />
    </aside>
  )
}

function ConceptEditor({ t, concept, onSaved }: { t: typeof copy.en; concept: ConceptRow | null; onSaved: (concept: Concept) => void }) {
  const [definition, setDefinition] = useState('')
  const [examples, setExamples] = useState('')
  const [pitfalls, setPitfalls] = useState('')
  const [notes, setNotes] = useState('')
  const [saved, setSaved] = useState(false)
  useEffect(() => {
    setDefinition(blocksToText(concept?.content?.definition))
    setExamples(blocksToText(concept?.content?.examples))
    setPitfalls(blocksToText(concept?.content?.pitfalls))
    setNotes(blocksToText(concept?.content?.notes))
  }, [concept?.id])
  if (!concept) return <aside className="concept-panel empty">{t.pickEdit}</aside>
  const activeConcept = concept
  async function save() {
    const updated = await api.request<Concept>(`/api/concepts/${activeConcept.id}/content`, {
      method: 'PATCH',
      body: JSON.stringify({
        definition: textToBlocks(definition),
        examples: textToBlocks(examples),
        pitfalls: textToBlocks(pitfalls),
        notes: textToBlocks(notes),
        source: 'manual',
      }),
    })
    onSaved(updated)
    setSaved(true)
    setTimeout(() => setSaved(false), 1200)
  }
  return (
    <aside className="editor-panel">
      <small>{concept.topic?.title}</small>
      <h2>{concept.term}</h2>
      <label>{t.definition}<textarea value={definition} onChange={(e) => setDefinition(e.target.value)} /></label>
      <label>{t.examples}<textarea value={examples} onChange={(e) => setExamples(e.target.value)} /></label>
      <label>{t.pitfalls}<textarea value={pitfalls} onChange={(e) => setPitfalls(e.target.value)} /></label>
      <label>{t.notes}<textarea value={notes} onChange={(e) => setNotes(e.target.value)} /></label>
      <button className="primary" onClick={save}><Save size={16} /> {saved ? t.saved : t.saveContent}</button>
    </aside>
  )
}

function RichContent({ t, content }: { t: typeof copy.en; content?: ConceptContent }) {
  if (!content) return <p className="muted">{t.noContent}</p>
  return (
    <div className="rich-content">
      <BlockGroup title={t.definition} blocks={content.definition} />
      <BlockGroup title={t.examples} blocks={content.examples} />
      <BlockGroup title={t.pitfalls} blocks={content.pitfalls} tone="warn" />
      <BlockGroup title={t.notes} blocks={content.notes} />
      <small className="source-line">{t.source}: {sourceLabel(content.source)}</small>
    </div>
  )
}

function BlockGroup({ title, blocks, tone = '' }: { title: string; blocks?: Block[] | null; tone?: string }) {
  if (!blocks?.length) return null
  return (
    <div className={`block-group ${tone}`}>
      <h4>{title}</h4>
      {blocks.map((block, index) => <p key={`${title}-${index}`}>{block.text}</p>)}
    </div>
  )
}

function Rating({ value, onRate }: { value: number; onRate: (rating: number) => void }) {
  return (
    <span className="rating" onClick={(event) => event.stopPropagation()}>
      {[0, 1, 2, 3, 4, 5].map((rating) => (
        <button key={rating} className={Math.round(value) === rating ? 'active' : ''} onClick={() => onRate(rating)} title={`${rating}/5`}>
          {rating}
        </button>
      ))}
    </span>
  )
}

function ChartCard({ title, icon, data, mode }: { title: string; icon: ReactNode; data: StatBucket[]; mode: 'reviews' | 'learned' | 'gain' }) {
  const values = data.map((row) => chartValue(row, mode))
  const max = Math.max(1, ...values)
  return (
    <div className="chart-card">
      <h3>{icon}{title}</h3>
      {data.length === 0 ? <p className="muted">No review history yet.</p> : (
        <div className={`bars ${mode}`}>
          {data.map((row) => {
            const value = chartValue(row, mode)
            return <div key={row.label} title={`${row.label}: ${formatChartValue(value, mode)}`}><i style={{ height: `${value > 0 ? Math.max(8, (value / max) * 100) : 0}%` }} /><span>{shortLabel(row.label)}</span></div>
          })}
        </div>
      )}
    </div>
  )
}

function WeakAreaCard({ title, icon, rows, empty }: { title: string; icon: ReactNode; rows: WeakArea[]; empty: string }) {
  return (
    <div className="insight-card">
      <h3>{icon}{title}</h3>
      {rows.length === 0 ? <p className="muted">{empty}</p> : (
        <div className="weak-list">
          {rows.map((row) => (
            <div key={row.label}>
              <span>{row.label}</span>
              <strong>{row.weak}</strong>
              <small>{row.averageMastery.toFixed(1)} / 5</small>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function Header({ eyebrow, title, action }: { eyebrow: string; title: string; action?: ReactNode }) {
  return (
    <div className="page-header">
      <div>
        <span>{eyebrow}</span>
        <h1>{title}</h1>
      </div>
      {action}
    </div>
  )
}

function Metric({ label, value, loading = false }: { label: string; value: number; loading?: boolean }) {
  return (
    <div className={`metric ${loading ? 'soft-loading' : ''}`}>
      <span>{label}</span>
      <strong>{loading ? '...' : Number.isInteger(value) ? value : value.toFixed(2)}</strong>
    </div>
  )
}

function NavButton({ active, collapsed, icon, label, onClick }: { active: boolean; collapsed: boolean; icon: ReactNode; label: string; onClick: () => void }) {
  return (
    <button className={active ? 'active' : ''} onClick={onClick} title={label}>
      {icon}
      {!collapsed && label}
    </button>
  )
}

function Avatar({ user }: { user: { name: string; avatarDataUrl?: string } }) {
  if (user.avatarDataUrl) return <img className="avatar" src={user.avatarDataUrl} alt="" />
  return <span className="avatar"><User size={17} />{initials(user.name)}</span>
}

function SetupCard({ children }: { children: ReactNode }) {
  return <div className="setup-card">{children}</div>
}

function KeyboardHint({ text }: { text: string }) {
  return <div className="keyboard-hint"><Keyboard size={15} /> {text}</div>
}

function ListSkeleton() {
  return <div className="table">{Array.from({ length: 10 }, (_, i) => <div className="concept-row skeleton-row" key={i} />)}</div>
}

function blocksToText(blocks?: Block[] | null) {
  return blocks?.map((block) => block.text).join('\n') ?? ''
}

function textToBlocks(text: string) {
  return text.split(/\n+/).map((line) => line.trim()).filter(Boolean).map((line) => ({ type: 'paragraph', text: line }))
}

function examCountdown(now = Date.now()) {
  const target = new Date('2026-05-12T12:00:00+08:00').getTime()
  const diff = Math.max(0, target - now)
  const days = Math.floor(diff / 86_400_000)
  const hours = Math.floor((diff % 86_400_000) / 3_600_000)
  const minutes = Math.floor((diff % 3_600_000) / 60_000)
  const seconds = Math.floor((diff % 60_000) / 1000)
  return { days, hours, minutes, seconds }
}

function cumulativeLearned(rows: StatBucket[]) {
  let total = 0
  return rows.map((row) => {
    total += row.learned
    return { ...row, learned: total }
  })
}

function chartValue(row: StatBucket, mode: 'reviews' | 'learned' | 'gain') {
  if (mode === 'reviews') return row.reviews
  if (mode === 'learned') return row.learned
  return row.masteryGain
}

function formatChartValue(value: number, mode: 'reviews' | 'learned' | 'gain') {
  return mode === 'gain' ? value.toFixed(2) : String(value)
}

function percent(value: number, total: number) {
  if (total <= 0) return 0
  return Math.round((value / total) * 100)
}

function shortLabel(label: string) {
  if (label.includes('-')) return label.slice(5)
  return label
}

function sourceLabel(source?: string) {
  const clean = (source ?? '').trim()
  if (!clean || clean === 'pending') return 'Awaiting Study Content'
  if (clean === 'unit0.md') return 'Unit 0 Key Terms'
  if (clean === 'unit1.md') return 'Unit 1 Study Notes'
  if (clean === 'ai-enrichment.compact') return 'AI AP Psych Notes'
  if (clean.includes('AP Psychology Notes')) return 'AP Psychology Notes'
  if (clean === 'manual') return 'Teacher Edited Notes'
  if (clean.startsWith('manual')) return 'Reviewed Local Notes'
  return clean.replace(/\.(md|txt|opml)$/i, '').replace(/[-_]/g, ' ')
}

function initials(name: string) {
  return name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]?.toUpperCase()).join('') || 'AP'
}

createRoot(document.getElementById('app')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
