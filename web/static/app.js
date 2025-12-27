// 仅保留“某周签到人员表格”相关逻辑。
// 数据来源：/api/stats（后端已按「同一人同一天只算一次」去重）。

let statsCache = null

// 周签到表格使用的数据
let weekMeta = {}         // weekKey -> { startDate, endDate, label }
let weeklyAttendance = {} // weekKey -> { person -> { dateStr: true } }
let allPersons = []       // 来自后端 stats.all_persons

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

function setHint(text) {
  const el = document.getElementById('hint')
  if (el) el.textContent = text
}

// 基于 statsCache 构造周签到表格数据
function buildWeeklyAttendanceFromStats() {
  if (!statsCache) return

  weekMeta = {}
  weeklyAttendance = {}
  allPersons = statsCache.all_persons || []

  const personDay = statsCache.person_day || []

  personDay.forEach((item) => {
    const person = (item.person || '').trim() || '未知'
    const date = (item.date || '').trim()
    if (!date) return

    const weekKey = getWeekKey(date)
    if (!weekKey) return

    if (!weekMeta[weekKey]) {
      const start = weekKey
      const end = addDays(weekKey, 6)
      weekMeta[weekKey] = { startDate: start, endDate: end, label: `${start} ~ ${end}` }
    }

    if (!weeklyAttendance[weekKey]) weeklyAttendance[weekKey] = {}
    if (!weeklyAttendance[weekKey][person]) weeklyAttendance[weekKey][person] = {}

    // 后端已做按天去重，这里只记录“是否来过”
    weeklyAttendance[weekKey][person][date] = true
  })

  // 兜底：如果后端没给 all_persons，就用统计里出现过的人员集合
  if (!allPersons || allPersons.length === 0) {
    const set = new Set()
    Object.keys(weeklyAttendance).forEach((wk) => {
      Object.keys(weeklyAttendance[wk] || {}).forEach((p) => set.add(p))
    })
    allPersons = Array.from(set).sort()
  }
}

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

function renderWeeklyTable() {
  const tbody = document.getElementById('weekTableBody')
  const weekSelect = document.getElementById('weekSelect')
  const weekKey = weekSelect.value

  tbody.innerHTML = ''

  if (!weekKey || !weeklyAttendance[weekKey]) {
    const tr = document.createElement('tr')
    tr.innerHTML = '<td colspan="8" class="px-4 py-6 text-center text-sm text-slate-500">该周暂无签到数据</td>'
    tbody.appendChild(tr)
    setHint('暂无数据。')
    return
  }

  const persons = (allPersons && allPersons.length > 0)
    ? allPersons
    : Object.keys(weeklyAttendance[weekKey] || {}).sort()

  if (!persons || persons.length === 0) {
    const tr = document.createElement('tr')
    tr.innerHTML = '<td colspan="8" class="px-4 py-6 text-center text-sm text-slate-500">该周暂无签到数据</td>'
    tbody.appendChild(tr)
    setHint('暂无数据。')
    return
  }

  persons.forEach((person, idx) => {
    const tr = document.createElement('tr')
    tr.className = idx % 2 === 0 ? 'bg-white' : 'bg-slate-50'

    const cells = []
    cells.push(`<td class="px-4 py-3 whitespace-nowrap text-sm font-medium text-slate-900">${person}</td>`)

    const personData = (weeklyAttendance[weekKey] && weeklyAttendance[weekKey][person]) || {}
    for (let i = 0; i < 7; i++) {
      const dateStr = addDays(weekKey, i)
      const hasSign = !!personData[dateStr]

      if (hasSign) {
        cells.push(
          `<td class="px-3 py-3 text-center">
            <span class="inline-flex items-center justify-center w-7 h-7 rounded-full bg-emerald-50 text-emerald-700 border border-emerald-200">✓</span>
          </td>`,
        )
      } else {
        cells.push('<td class="px-3 py-3 text-center text-slate-400">—</td>')
      }
    }

    tr.innerHTML = cells.join('')
    tbody.appendChild(tr)
  })

  const start = weekMeta[weekKey]?.startDate || weekKey
  const end = weekMeta[weekKey]?.endDate || addDays(weekKey, 6)
  setHint(`展示周：${start} ~ ${end}（同一人同一天只算一次）`)
}

async function fetchStats() {
  try {
    setHint('加载中…')
    const res = await fetch('/api/stats')
    const stats = await res.json()
    statsCache = stats

    buildWeeklyAttendanceFromStats()
    initWeekSelect()
    renderWeeklyTable()
  } catch (e) {
    console.error('获取统计失败', e)
    setHint('加载失败：请检查后端服务与 /api/stats。')
  }
}

document.addEventListener('DOMContentLoaded', () => {
  const refreshBtn = document.getElementById('refreshBtn')
  const weekSelect = document.getElementById('weekSelect')

  refreshBtn.addEventListener('click', () => {
    fetchStats()
  })

  weekSelect.addEventListener('change', () => {
    renderWeeklyTable()
  })

  fetchStats()
})
