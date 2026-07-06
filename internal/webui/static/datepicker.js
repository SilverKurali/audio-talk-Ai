// Cascading date picker - only shows dates with records
let dpDateStats = null;
let dpMode = '';
let dpSelectedYear = '';
let dpSelectedMonth = '';

function initDatePicker() {
  if (!dpDateStats) {
    api('GET', '/history/dates').then(d => { dpDateStats = d; });
  }
  document.addEventListener('click', (e) => {
    const popup = document.getElementById('date-picker-popup');
    if (popup && popup.style.display === 'block' && !popup.contains(e.target) && !e.target.classList.contains('date-display')) {
      popup.style.display = 'none';
    }
  });
}

function openDatePicker(mode) {
  if (!dpDateStats) {
    api('GET', '/history/dates').then(d => {
      dpDateStats = d;
      showDatePicker(mode);
    });
  } else {
    showDatePicker(mode);
  }
}

function showDatePicker(mode) {
  dpMode = mode;
  dpSelectedYear = '';
  dpSelectedMonth = '';
  const popup = document.getElementById('date-picker-popup');
  popup.style.display = 'block';

  const display = document.getElementById(mode === 'from' ? 'date-from-display' : 'date-to-display');
  const rect = display.getBoundingClientRect();
  let left = rect.left;
  let top = rect.bottom + 6;
  if (left + 280 > window.innerWidth) left = window.innerWidth - 290;
  if (top + 300 > window.innerHeight) top = rect.top - 310;
  if (top < 10) top = 10;
  popup.style.left = left + 'px';
  popup.style.top = top + 'px';

  const years = Object.keys(dpDateStats).sort().reverse();
  const yearsEl = document.getElementById('dp-years');
  const monthsEl = document.getElementById('dp-months');
  const daysEl = document.getElementById('dp-days');
  monthsEl.style.display = 'none';
  daysEl.style.display = 'none';

  yearsEl.innerHTML = '<div class="dp-label">年份</div>' + years.map(y =>
    `<div class="dp-item" onclick="selectYear('${y}')">${y}</div>`
  ).join('');
  if (years.length === 0) {
    yearsEl.innerHTML = '<div class="dp-label" style="color:#ef5350">暂无记录</div>';
  }
}

function selectYear(y) {
  dpSelectedYear = y;
  dpSelectedMonth = '';
  const months = Object.keys(dpDateStats[y]).sort();
  const names = {'01':'1月','02':'2月','03':'3月','04':'4月','05':'5月','06':'6月','07':'7月','08':'8月','09':'9月','10':'10月','11':'11月','12':'12月'};
  const monthsEl = document.getElementById('dp-months');
  const daysEl = document.getElementById('dp-days');
  daysEl.style.display = 'none';
  monthsEl.style.display = 'flex';
  monthsEl.innerHTML = '<div class="dp-label">月份</div>' + months.map(m =>
    `<div class="dp-item" onclick="selectMonth('${m}')">${names[m] || m}</div>`
  ).join('');
}

function selectMonth(m) {
  dpSelectedMonth = m;
  const days = dpDateStats[dpSelectedYear][m];
  const daysEl = document.getElementById('dp-days');
  daysEl.style.display = 'flex';
  daysEl.innerHTML = '<div class="dp-label">日期</div>' + days.map(d => {
    const dayStr = String(d).padStart(2, '0');
    return `<div class="dp-item" onclick="selectDay('${dayStr}')">${d}日</div>`;
  }).join('');
}

function selectDay(d) {
  const fullDate = dpSelectedYear + '-' + dpSelectedMonth + '-' + d;
  const displayDate = dpSelectedYear + '/' + dpSelectedMonth + '/' + d;
  const display = document.getElementById(dpMode === 'from' ? 'date-from-display' : 'date-to-display');
  display.textContent = displayDate;
  display.dataset.date = fullDate;
  document.getElementById('date-picker-popup').style.display = 'none';
}
