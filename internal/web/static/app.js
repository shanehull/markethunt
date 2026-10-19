/* Markethunt: htmx handles sorting, filtering, and the drawer; Chart.js draws
 * the country history charts. This file closes the drawer, keeps it
 * keyboard-accessible, keeps the filter controls in step with the URL, and
 * builds the charts after each drawer swap. */
(function () {
  const ACCENT = '#4f6bff';
  const GRID = 'rgba(120, 128, 150, 0.14)';
  const TICK = '#8a90a4';
  let charts = [];

  function drawer() {
    return document.getElementById('drawer');
  }

  function close() {
    const d = drawer();
    if (!d) return;
    destroyCharts();
    d.innerHTML = '';
    d.classList.remove('open');
  }

  function destroyCharts() {
    charts.forEach(function (c) {
      try {
        c.destroy();
      } catch (e) {
        /* already gone */
      }
    });
    charts = [];
  }

  function yearLabel(iso) {
    const d = new Date(iso);
    return isNaN(d) ? iso : String(d.getUTCFullYear());
  }

  function initCharts(root) {
    if (typeof Chart === 'undefined') return;
    const payload = root.querySelector('[data-charts]');
    if (!payload) return;
    let data;
    try {
      data = JSON.parse(payload.dataset.charts);
    } catch (e) {
      return;
    }
    data.forEach(function (chart) {
      const canvas = root.querySelector('#drawer-chart-' + chart.id);
      if (!canvas) return;
      const unit = chart.unit || '';
      charts.push(new Chart(canvas, {
        type: 'line',
        data: {
          labels: chart.points.map(function (p) { return yearLabel(p.date); }),
          datasets: [{
            label: chart.label,
            data: chart.points.map(function (p) { return p.value; }),
            borderColor: ACCENT,
            backgroundColor: 'rgba(79, 107, 255, 0.12)',
            borderWidth: 2,
            fill: true,
            pointRadius: 2,
            pointHoverRadius: 5,
            tension: 0.2,
          }],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          interaction: { mode: 'index', intersect: false },
          plugins: {
            legend: { display: false },
            tooltip: {
              displayColors: false,
              callbacks: {
                label: function (ctx) {
                  return unit ? ctx.parsed.y + unit : String(ctx.parsed.y);
                },
              },
            },
          },
          scales: {
            x: {
              grid: { display: false },
              ticks: { color: TICK, maxRotation: 0, autoSkip: true, maxTicksLimit: 5, font: { size: 10 } },
            },
            y: {
              grid: { color: GRID },
              ticks: { color: TICK, maxTicksLimit: 4, font: { size: 10 } },
            },
          },
        },
      }));
    });
  }

  document.addEventListener('click', function (e) {
    if (e.target.closest('[data-drawer-close]')) {
      close();
      return;
    }
    const d = drawer();
    if (d && e.target === d) close();
  });

  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') close();
    if (e.key === 'Enter') {
      const row = e.target.closest('tr.row');
      if (row) row.click();
    }
  });

  // After the table swaps, mirror the URL back into the filter controls so a
  // search or region change keeps the active sort.
  function syncControls() {
    const params = new URLSearchParams(window.location.search);
    const form = document.getElementById('filters');
    if (!form) return;
    set(form, 'input[name=sort]', params.get('sort'));
    set(form, 'input[name=dir]', params.get('dir'));
    set(form, 'select[name=region]', params.get('region') || '');
    const q = form.querySelector('input[name=q]');
    if (q && params.get('q') !== null && q.value !== params.get('q')) {
      q.value = params.get('q');
    }
  }

  function set(form, selector, value) {
    if (value === null) return;
    const el = form.querySelector(selector);
    if (el) el.value = value;
  }

  document.addEventListener('htmx:beforeSwap', function (e) {
    if (e.target && e.target.id === 'drawer') destroyCharts();
  });

  document.addEventListener('htmx:afterSwap', function (e) {
    if (!e.target) return;
    if (e.target.id === 'drawer') {
      e.target.classList.add('open');
      initCharts(e.target);
      const closeBtn = e.target.querySelector('.drawer-close');
      if (closeBtn) closeBtn.focus();
    }
    if (e.target.id === 'rank') syncControls();
  });
})();
