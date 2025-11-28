let currentPage = 1
const pageSize = 20
let totalPages = 1
let currentPageData = []

let statsCache = null
let weeklySignChart = null
let monthlySignChart = null

// 周 / 月 / 周签到表格 使用的衍生数据
let weekMeta = {}           // weekKey -> { startDate, endDate, label }
let weeklyPeopleData = []   // [{ weekKey, label, count }]
let monthlyPeopleData = []  // [{ month, count }]
let weeklyAttendance = {}   // weekKey -> { person -> { dateStr: true } }
let allPersons = []         // 来自后端 stats.all_persons

// 防抖
function debounce(fn, delay) {
  let timer = null
  return function (...args) {
    clearTimeout(timer)
    timer = setTimeout(() => fn.apply(this, args), delay)
  }
}

// 状态徽章（日志用）
function renderStatusBadge(status) {
  const base =
    'inline-flex items-center px-2 py-[2px] rounded-full text-[11px] font-medium border'
  const s = (status ?? '').toString()
  const upper = s.toUpperCase()

  switch (upper) {
    case 'MATCH':
      return `<span class="${base} bg-emerald-500/10 text-emerald-300 border-emerald-500/40">MATCH</span>`
    case 'ERROR':
      return `<span class="${base} bg-rose-500/10 text-rose-300 border-rose-500/40">ERROR</span>`
    case 'NO_FACE':
      return `<span class="${base} bg-amber-500/10 text-amber-300 border-amber-500/40">NO_FACE</span>`
    case '':
      return `<span class="${base} bg-slate-500/10 text-slate-300 border-slate-500/40">空</span>`
    default:
      return `<span class="${base} bg-slate-500/10 text-slate-300 border-slate-500/40">${s}</span>`
  }
}

// 根据 yyyy-mm-dd 计算“周一”的日期字符串作为 weekKey
function getWeekKey(dateStr) {
  if (!dateStr) return ''
  const parts = dateStr.split('-').map(Number)
  if (parts.length < 3) return ''
  const [y, m, d] = parts
  const dt = new Date(y, m - 1, d)
  const day = dt.getDay() // 0=周日,1=周一,...,6=周六
  const offset = (day + 6) % 7 // 转成“周一=0”的偏移
  dt.setDate(dt.getDate() - offset)
  const yy = dt.getFullYear()
  const mm = String(dt.getMonth() + 1).padStart(2, '0')
  const dd = String(dt.getDate()).padStart(2, '0')
  return `${yy}-${mm}-${dd}`
}

// 在 yyyy-mm-dd 上加 days 天
function addDays(dateStr, days) {
  const parts = dateStr.split('-').map(Number)
  if (parts.length < 3) return dateStr
  const [y, m, d] = parts
  const dt = new Date(y, m - 1, d)
  dt.setDate(dt.getDate() + days)
  const yy = dt.getFullYear()
  const mm = String(dt.getMonth() + 1).padStart(2, '0')
  const dd = String(dt.getDate()).padStart(2, '0')
  return `${yy}-${mm}-${dd}`
}

// 基于 statsCache 计算周 / 月统计和周签到表格数据
function buildDerivedFromStats() {
  if (!statsCache) return

  weekMeta = {}
  weeklyPeopleData = []
  monthlyPeopleData = []
  weeklyAttendance = {}
  allPersons = statsCache.all_persons || []

  const dayPeople = statsCache.day_people || []
  const personDay = statsCache.person_day || []

  const weeklyPeopleMap = {} // weekKey -> sum(people)
  const monthlyMap = {}      // yyyy-mm -> sum(people)

  // 1) 从每天有效访客人数 day_people 构造“周”和“月”的人数汇总
  dayPeople.forEach((item) => {
    const date = item.date || ''
    const people = item.people || 0
    if (!date) return

    const weekKey = getWeekKey(date)
    if (weekKey) {
      if (!weeklyPeopleMap[weekKey]) {
        weeklyPeopleMap[weekKey] = 0
      }
      weeklyPeopleMap[weekKey] += people

      if (!weekMeta[weekKey]) {
        const start = weekKey
        const end = addDays(weekKey, 6)
        weekMeta[weekKey] = {
          startDate: start,
          endDate: end,
          label: `${start} ~ ${end}`,
        }
      }
    }

    const month = date.slice(0, 7)
    if (month) {
      if (!monthlyMap[month]) {
        monthlyMap[month] = 0
      }
      monthlyMap[month] += people
    }
  })

  weeklyPeopleData = Object.keys(weeklyPeopleMap)
    .sort()
    .map((weekKey) => ({
      weekKey,
      label: weekMeta[weekKey]?.label || weekKey,
      count: weeklyPeopleMap[weekKey],
    }))

  monthlyPeopleData = Object.keys(monthlyMap)
    .sort()
    .map((month) => ({
      month,
      count: monthlyMap[month],
    }))

  // 2) 基于 person_day 构造“某周签到人员表格”数据
  personDay.forEach((item) => {
    const person = item.person || '未知'
    const date = item.date || ''
    if (!date || !person) return

    const weekKey = getWeekKey(date)
    if (!weekKey) return

    if (!weekMeta[weekKey]) {
      const start = weekKey
      const end = addDays(weekKey, 6)
      weekMeta[weekKey] = {
        startDate: start,
        endDate: end,
        label: `${start} ~ ${end}`,
      }
    }

    if (!weeklyAttendance[weekKey]) {
      weeklyAttendance[weekKey] = {}
    }
    if (!weeklyAttendance[weekKey][person]) {
      weeklyAttendance[weekKey][person] = {}
    }

    // 按天去重逻辑在后端已经做过，这里只关心“是否来过”
    weeklyAttendance[weekKey][person][date] = true
  })

  // 如果后端没有返回 all_persons，则退化为用 person_day 里出现的人员集合
  if (!allPersons || allPersons.length === 0) {
    const set = new Set()
    Object.keys(weeklyAttendance).forEach((wk) => {
      Object.keys(weeklyAttendance[wk]).forEach((p) => set.add(p))
    })
    allPersons = Array.from(set).sort()
  }
}

// 初始化 / 更新周选择下拉框
function initWeekSelect() {
  const weekSelect = document.getElementById('weekSelect')
  const weekKeys = Object.keys(weekMeta).sort()

  if (weekKeys.length === 0) {
    weekSelect.innerHTML = '<option value="">暂无周数据</option>'
    return
  }

  weekSelect.innerHTML = ''
  weekKeys.forEach((wk) => {
    const opt = document.createElement('option')
    opt.value = wk
    opt.textContent = weekMeta[wk]?.label || wk
    weekSelect.appendChild(opt)
  })

  // 默认选最后一周（最新）
  weekSelect.value = weekKeys[weekKeys.length - 1]
}

// 图表：每周签到人数
function updateWeeklySignChart() {
  const ctx = document.getElementById('weeklySignChart').getContext('2d')

  if (!weeklyPeopleData || weeklyPeopleData.length === 0) {
    if (weeklySignChart) weeklySignChart.destroy()
    weeklySignChart = new Chart(ctx, {
      type: 'line',
      data: {
        labels: [],
        datasets: [
          {
            label: '暂无数据',
            data: [],
            borderWidth: 1.5,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { labels: { color: '#e5e7eb', font: { size: 11 } } },
        },
        scales: {
          x: {
            ticks: { color: '#9ca3af' },
            grid: { display: false },
          },
          y: {
            ticks: { color: '#9ca3af' },
            grid: { color: 'rgba(55,65,81,0.5)' },
            beginAtZero: true,
          },
        },
      },
    })
    return
  }

  const labels = weeklyPeopleData.map((d) => d.label)
  const counts = weeklyPeopleData.map((d) => d.count)

  if (weeklySignChart) weeklySignChart.destroy()

  weeklySignChart = new Chart(ctx, {
    type: 'line',
    data: {
      labels,
      datasets: [
        {
          label: '每周有效签到人数（按天去重）',
          data: counts,
          borderWidth: 1.5,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: 'index', intersect: false },
      plugins: {
        legend: { labels: { color: '#e5e7eb', font: { size: 11 } } },
      },
      scales: {
        x: {
          ticks: { color: '#9ca3af', maxRotation: 45, minRotation: 0 },
          grid: { display: false },
        },
        y: {
          ticks: { color: '#9ca3af' },
          grid: { color: 'rgba(55,65,81,0.5)' },
          beginAtZero: true,
        },
      },
    },
  })
}

// 图表：每月签到人数
function updateMonthlySignChart() {
  const ctx = document.getElementById('monthlySignChart').getContext('2d')

  if (!monthlyPeopleData || monthlyPeopleData.length === 0) {
    if (monthlySignChart) monthlySignChart.destroy()
    monthlySignChart = new Chart(ctx, {
      type: 'line',
      data: {
        labels: [],
        datasets: [
          {
            label: '暂无数据',
            data: [],
            borderWidth: 1.5,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { labels: { color: '#e5e7eb', font: { size: 11 } } },
        },
        scales: {
          x: {
            ticks: { color: '#9ca3af' },
            grid: { display: false },
          },
          y: {
            ticks: { color: '#9ca3af' },
            grid: { color: 'rgba(55,65,81,0.5)' },
            beginAtZero: true,
          },
        },
      },
    })
    return
  }

  const labels = monthlyPeopleData.map((d) => d.month)
  const counts = monthlyPeopleData.map((d) => d.count)

  if (monthlySignChart) monthlySignChart.destroy()

  monthlySignChart = new Chart(ctx, {
    type: 'line',
    data: {
      labels,
      datasets: [
        {
          label: '每月有效签到人数（按天去重）',
          data: counts,
          borderWidth: 1.5,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: 'index', intersect: false },
      plugins: {
        legend: { labels: { color: '#e5e7eb', font: { size: 11 } } },
      },
      scales: {
        x: {
          ticks: { color: '#9ca3af', maxRotation: 45, minRotation: 0 },
          grid: { display: false },
        },
        y: {
          ticks: { color: '#9ca3af' },
          grid: { color: 'rgba(55,65,81,0.5)' },
          beginAtZero: true,
        },
      },
    },
  })
}

// 渲染“某周签到人员表格”
function renderWeeklyTable() {
  const tbody = document.getElementById('weekTableBody')
  const weekSelect = document.getElementById('weekSelect')
  const weekKey = weekSelect.value

  tbody.innerHTML = ''

  if (!weekKey || !weeklyAttendance[weekKey]) {
    const tr = document.createElement('tr')
    tr.innerHTML =
      '<td colspan="8" class="px-3 py-4 text-center text-sm text-slate-500">该周暂无签到数据</td>'
    tbody.appendChild(tr)
    return
  }

  const persons = allPersons && allPersons.length > 0
    ? allPersons
    : Object.keys(weeklyAttendance[weekKey]).sort()

  if (!persons || persons.length === 0) {
    const tr = document.createElement('tr')
    tr.innerHTML =
      '<td colspan="8" class="px-3 py-4 text-center text-sm text-slate-500">该周暂无签到数据</td>'
    tbody.appendChild(tr)
    return
  }

  persons.forEach((person) => {
    const tr = document.createElement('tr')
    tr.className = 'hover:bg-slate-800/60 transition-colors'
    const cells = []

    // 姓名
    cells.push(
      `<td class="px-3 py-2 whitespace-nowrap text-sm text-slate-100">${person}</td>`,
    )

    // 周一到周日
    const personData = (weeklyAttendance[weekKey] && weeklyAttendance[weekKey][person]) || {}
    for (let i = 0; i < 7; i++) {
      const dateStr = addDays(weekKey, i)
      const hasSign = !!personData[dateStr]

      if (hasSign) {
        cells.push(
          `<td class="px-3 py-2 text-center text-xs">
            <span class="inline-flex items-center justify-center px-2 py-1 rounded-full bg-emerald-500/10 text-emerald-300 border border-emerald-500/40">
              ✓
            </span>
          </td>`,
        )
      } else {
        cells.push(
          '<td class="px-3 py-2 text-center text-xs text-slate-500">—</td>',
        )
      }
    }

    tr.innerHTML = cells.join('')
    tbody.appendChild(tr)
  })
}

// 获取统计数据
async function fetchStats() {
  try {
    const res = await fetch('/api/stats')
    const stats = await res.json()
    statsCache = stats

    document.getElementById('totalCount').textContent = stats.total ?? 0
    document.getElementById('validCount').textContent = stats.valid ?? 0
    document.getElementById('errorCount').textContent = stats.error ?? 0
    document.getElementById('noFaceCount').textContent = stats.no_face ?? 0

    // 构建周/月/周表格的衍生数据
    buildDerivedFromStats()
    // 初始化周下拉
    initWeekSelect()
    // 更新图表 与 周签到表格
    updateWeeklySignChart()
    updateMonthlySignChart()
    renderWeeklyTable()
  } catch (e) {
    console.error('获取统计失败', e)
  }
}

// 列表数据（简化版原始记录）
async function fetchData() {
  const status = document.getElementById('statusFilter').value
  const q = document.getElementById('searchInput').value.trim()

  const params = new URLSearchParams()
  params.set('page', String(currentPage))
  params.set('pageSize', String(pageSize))
  if (status) params.set('status', status)
  if (q) params.set('q', q)

  try {
    const res = await fetch('/api/records?' + params.toString())
    const json = await res.json()
    currentPageData = json.data || []
    renderTable(currentPageData)
    updateSummary(json)
  } catch (e) {
    console.error('获取列表失败', e)
  }
}

function renderTable(rows) {
  const tbody = document.getElementById('tableBody')
  tbody.innerHTML = ''

  if (!rows || rows.length === 0) {
    const tr = document.createElement('tr')
    tr.innerHTML =
      '<td colspan="3" class="px-3 py-4 text-center text-sm text-slate-500">暂无数据</td>'
    tbody.appendChild(tr)
    return
  }

  rows.forEach((rec) => {
    const tr = document.createElement('tr')
    tr.className = 'hover:bg-slate-800/60 transition-colors'
    tr.innerHTML = `
      <td class="px-3 py-2 whitespace-nowrap text-xs text-slate-400">
        ${rec.timestamp || ''}
      </td>
      <td class="px-3 py-2 whitespace-nowrap text-sm">
        ${rec.match_name || '-'}
      </td>
      <td class="px-3 py-2 whitespace-nowrap">
        ${renderStatusBadge(rec.status)}
      </td>
    `
    tbody.appendChild(tr)
  })
}

function updateSummary(meta) {
  const summary = document.getElementById('summary')
  const prevBtn = document.getElementById('prevPage')
  const nextBtn = document.getElementById('nextPage')

  const total = meta.total || 0
  const page = meta.page || 1
  const size = meta.pageSize || pageSize

  totalPages = Math.max(1, Math.ceil(total / size))

  summary.textContent = `共 ${total} 条记录 · 第 ${page} / ${totalPages} 页`

  prevBtn.disabled = page <= 1
  nextBtn.disabled = page >= totalPages
}

document.addEventListener('DOMContentLoaded', () => {
  const searchInput = document.getElementById('searchInput')
  const statusFilter = document.getElementById('statusFilter')
  const prevPage = document.getElementById('prevPage')
  const nextPage = document.getElementById('nextPage')
  const refreshBtn = document.getElementById('refreshBtn')
  const weekSelect = document.getElementById('weekSelect')

  // 搜索（姓名 / 状态）
  searchInput.addEventListener(
    'input',
    debounce(() => {
      currentPage = 1
      fetchData()
    }, 300),
  )

  // 状态筛选
  statusFilter.addEventListener('change', () => {
    currentPage = 1
    fetchData()
  })

  // 分页：上一页
  prevPage.addEventListener('click', () => {
    if (currentPage > 1) {
      currentPage--
      fetchData()
    }
  })

  // 分页：下一页
  nextPage.addEventListener('click', () => {
    if (currentPage < totalPages) {
      currentPage++
      fetchData()
    }
  })

  // 刷新按钮：重新拉取统计和列表
  refreshBtn.addEventListener('click', () => {
    fetchStats()
    fetchData()
  })

  // 周选择：切换周签到表
  weekSelect.addEventListener('change', () => {
    renderWeeklyTable()
  })

  // 首次加载
  fetchStats()
  fetchData()
})
