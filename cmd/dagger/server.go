package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DAGViewModel holds DAG metadata along with precomputed Mermaid diagrams.
type DAGViewModel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Package   string `json:"package"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Location  string `json:"location"`
	MermaidTD string `json:"mermaidTD"`
	MermaidLR string `json:"mermaidLR"`
}

func prepareViewModels(dags []*DiscoveredDAG) []DAGViewModel {
	models := make([]DAGViewModel, len(dags))
	for i, d := range dags {
		models[i] = DAGViewModel{
			ID:        d.ID,
			Name:      d.Name,
			Package:   d.Package,
			File:      d.File,
			Line:      d.Line,
			Location:  fmt.Sprintf("%s:%d", d.File, d.Line),
			MermaidTD: GenerateMermaid(d.RootExpr, "TD"),
			MermaidLR: GenerateMermaid(d.RootExpr, "LR"),
		}
	}
	return models
}

func createMux(dags []*DiscoveredDAG, defaultOrientation string) http.Handler {
	mux := http.NewServeMux()
	viewModels := prepareViewModels(dags)

	if strings.ToUpper(defaultOrientation) != "LR" {
		defaultOrientation = "TD"
	} else {
		defaultOrientation = "LR"
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		dataJSON, err := json.Marshal(viewModels)
		if err != nil {
			http.Error(w, "failed to marshal DAGs data", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl, err := template.New("dashboard").Parse(dashboardHTML)
		if err != nil {
			http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		tmplData := struct {
			DAGsJSON           template.JS
			DAGCount           int
			DefaultOrientation string
			FirstDAGName       string
		}{
			DAGsJSON:           template.JS(dataJSON), //nolint:gosec // G203: dataJSON is produced by json.Marshal from AST models, not user input
			DAGCount:           len(viewModels),
			DefaultOrientation: defaultOrientation,
		}
		if len(viewModels) > 0 {
			tmplData.FirstDAGName = viewModels[0].Name
		}

		_ = tmpl.Execute(w, tmplData)
	})

	mux.HandleFunc("/api/dags", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(viewModels)
	})

	mux.HandleFunc("/api/mermaid", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		idOrIdx := query.Get("select")
		if idOrIdx == "" {
			idOrIdx = query.Get("id")
		}
		ori := query.Get("orientation")
		if strings.ToUpper(ori) != "LR" {
			ori = "TD"
		} else {
			ori = "LR"
		}

		if len(viewModels) == 0 {
			http.Error(w, "no DAGs available", http.StatusNotFound)
			return
		}

		selected := viewModels[0]
		if idOrIdx != "" {
			found := false
			if idx, err := strconv.Atoi(idOrIdx); err == nil && idx >= 0 && idx < len(viewModels) {
				selected = viewModels[idx]
				found = true
			} else {
				for _, vm := range viewModels {
					if strings.EqualFold(vm.ID, idOrIdx) || strings.EqualFold(vm.Name, idOrIdx) {
						selected = vm
						found = true
						break
					}
				}
			}
			if !found {
				http.Error(w, "DAG not found", http.StatusNotFound)
				return
			}
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if ori == "LR" {
			_, _ = w.Write([]byte(selected.MermaidLR))
		} else {
			_, _ = w.Write([]byte(selected.MermaidTD))
		}
	})

	return mux
}

func startServer(addr string, dags []*DiscoveredDAG, defaultOrientation string) error {
	handler := createMux(dags, defaultOrientation)
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Printf("dagger visualizer serving %d DAG(s) at http://%s\n", len(dags), addr)
	return server.ListenAndServe()
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Dagger — Interactive DAG Visualizer</title>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/mermaid/10.9.0/mermaid.min.js"></script>
  <style>
    :root {
      --bg: #0f172a;
      --card-bg: #1e293b;
      --card-border: #334155;
      --text: #f8fafc;
      --text-muted: #94a3b8;
      --accent: #38bdf8;
      --accent-hover: #0ea5e9;
      --canvas-bg: #f8fafc;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      background: var(--bg);
      color: var(--text);
      min-height: 100vh;
      display: flex;
      flex-direction: column;
    }
    header {
      background: var(--card-bg);
      border-bottom: 1px solid var(--card-border);
      padding: 12px 24px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;
      flex-wrap: wrap;
    }
    .brand {
      display: flex;
      align-items: center;
      gap: 10px;
      font-weight: 700;
      font-size: 1.25rem;
      color: var(--text);
    }
    .badge {
      background: #0369a1;
      color: #e0f2fe;
      font-size: 0.75rem;
      padding: 2px 8px;
      border-radius: 12px;
      font-weight: 600;
    }
    .controls {
      display: flex;
      align-items: center;
      gap: 12px;
      flex-wrap: wrap;
    }
    select {
      background: #0f172a;
      color: var(--text);
      border: 1px solid var(--card-border);
      padding: 8px 12px;
      border-radius: 6px;
      font-size: 0.875rem;
      max-width: 380px;
      outline: none;
      cursor: pointer;
    }
    select:focus {
      border-color: var(--accent);
    }
    button {
      background: #334155;
      color: var(--text);
      border: 1px solid var(--card-border);
      padding: 8px 14px;
      border-radius: 6px;
      font-size: 0.875rem;
      cursor: pointer;
      display: inline-flex;
      align-items: center;
      gap: 6px;
      transition: all 0.15s ease;
    }
    button:hover {
      background: #475569;
    }
    button.primary {
      background: var(--accent);
      color: #0f172a;
      font-weight: 600;
      border: none;
    }
    button.primary:hover {
      background: var(--accent-hover);
    }
    .meta-bar {
      background: #1e293b;
      border-bottom: 1px solid var(--card-border);
      padding: 10px 24px;
      display: flex;
      align-items: center;
      gap: 24px;
      font-size: 0.85rem;
      flex-wrap: wrap;
    }
    .meta-item {
      display: flex;
      align-items: center;
      gap: 6px;
    }
    .meta-label {
      color: var(--text-muted);
    }
    .meta-value {
      color: var(--text);
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      font-weight: 500;
    }
    main {
      flex: 1;
      display: flex;
      flex-direction: column;
      padding: 20px;
      overflow: hidden;
    }
    .canvas-container {
      flex: 1;
      background: var(--canvas-bg);
      border-radius: 8px;
      border: 1px solid var(--card-border);
      position: relative;
      overflow: hidden;
      min-height: 500px;
      cursor: grab;
      user-select: none;
    }
    .canvas-container.grabbing {
      cursor: grabbing;
    }
    #diagram-stage {
      position: absolute;
      top: 0;
      left: 0;
      transform-origin: 0 0;
      will-change: transform;
    }
    #diagram svg {
      display: block;
      max-width: none !important;
    }
    .canvas-controls {
      position: absolute;
      bottom: 16px;
      right: 16px;
      background: rgba(30, 41, 59, 0.9);
      backdrop-filter: blur(8px);
      border: 1px solid var(--card-border);
      border-radius: 8px;
      padding: 4px;
      display: flex;
      align-items: center;
      gap: 4px;
      box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.3);
      z-index: 10;
    }
    .canvas-controls button {
      padding: 6px 10px;
      background: transparent;
      border: none;
      color: var(--text);
      font-size: 0.8rem;
      border-radius: 4px;
      cursor: pointer;
    }
    .canvas-controls button:hover {
      background: #334155;
    }
    .canvas-controls .zoom-indicator {
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      min-width: 52px;
      text-align: center;
      font-weight: 600;
      font-size: 0.75rem;
    }
    .modal-overlay {
      position: fixed;
      inset: 0;
      background: rgba(15, 23, 42, 0.8);
      display: none;
      align-items: center;
      justify-content: center;
      z-index: 100;
      padding: 20px;
    }
    .modal-overlay.active {
      display: flex;
    }
    .modal {
      background: var(--card-bg);
      border: 1px solid var(--card-border);
      border-radius: 8px;
      width: 100%;
      max-width: 700px;
      max-height: 80vh;
      display: flex;
      flex-direction: column;
      box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.5);
    }
    .modal-header {
      padding: 16px 20px;
      border-bottom: 1px solid var(--card-border);
      display: flex;
      justify-content: space-between;
      align-items: center;
    }
    .modal-body {
      padding: 20px;
      overflow: auto;
      flex: 1;
    }
    pre {
      background: #0f172a;
      border-radius: 6px;
      padding: 16px;
      color: #e2e8f0;
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      font-size: 0.85rem;
      line-height: 1.5;
      overflow-x: auto;
      white-space: pre-wrap;
    }
    .toast {
      position: fixed;
      bottom: 24px;
      right: 24px;
      background: #10b981;
      color: #ffffff;
      padding: 10px 18px;
      border-radius: 6px;
      font-size: 0.875rem;
      font-weight: 500;
      box-shadow: 0 10px 15px -3px rgba(0,0,0,0.3);
      opacity: 0;
      transform: translateY(10px);
      transition: all 0.2s ease;
      pointer-events: none;
      z-index: 200;
    }
    .toast.show {
      opacity: 1;
      transform: translateY(0);
    }
    .empty-state {
      color: #475569;
      text-align: center;
      font-size: 1.1rem;
    }
  </style>
</head>
<body>
  <header>
    <div class="brand">
      <span>dagger</span>
      <span class="badge">{{ .DAGCount }} DAGs</span>
    </div>

    <div class="controls">
      <select id="dagSelector" onchange="onDAGSelect(this.value)">
        <!-- options populated by JS -->
      </select>

      <button id="orientBtn" onclick="toggleOrientation()">
        <span>Orientation: <strong id="orientLabel">{{ .DefaultOrientation }}</strong></span>
      </button>

      <button id="copyBtn" onclick="copyMermaid()">
        <svg width="15" height="15" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
          <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
        </svg>
        Copy Mermaid
      </button>

      <button onclick="toggleRawModal(true)">
        <svg width="15" height="15" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <polyline points="16 18 22 12 16 6"></polyline>
          <polyline points="8 6 2 12 8 18"></polyline>
        </svg>
        Raw Mermaid
      </button>
    </div>
  </header>

  <div class="meta-bar">
    <div class="meta-item">
      <span class="meta-label">DAG:</span>
      <span class="meta-value" id="metaName">—</span>
    </div>
    <div class="meta-item">
      <span class="meta-label">Package:</span>
      <span class="meta-value" id="metaPkg">—</span>
    </div>
    <div class="meta-item">
      <span class="meta-label">Location:</span>
      <span class="meta-value" id="metaLoc">—</span>
    </div>
  </div>

  <main>
    <div class="canvas-container" id="canvasContainer">
      <div class="canvas-controls">
        <button onclick="zoomIn()" title="Zoom In (+)">
          <svg width="15" height="15" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><line x1="12" y1="5" x2="12" y2="19"></line><line x1="5" y1="12" x2="19" y2="12"></line></svg>
        </button>
        <button class="zoom-indicator" onclick="resetZoom()" title="Reset Zoom (100%)" id="zoomLevel">100%</button>
        <button onclick="zoomOut()" title="Zoom Out (-)">
          <svg width="15" height="15" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><line x1="5" y1="12" x2="19" y2="12"></line></svg>
        </button>
        <button onclick="fitToScreen()" title="Fit to Screen (Double Click Canvas)">
          <svg width="15" height="15" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M8 3H5a2 2 0 0 0-2 2v3m18 0V5a2 2 0 0 0-2-2h-3m0 18h3a2 2 0 0 0 2-2v-3M3 16v3a2 2 0 0 0 2 2h3"/></svg>
        </button>
      </div>
      <div id="diagram-stage">
        <div id="diagram">
          <div class="empty-state">Loading diagram...</div>
        </div>
      </div>
    </div>
  </main>

  <div class="modal-overlay" id="rawModal" onclick="if(event.target===this) toggleRawModal(false)">
    <div class="modal">
      <div class="modal-header">
        <h3 id="modalTitle">Raw Mermaid</h3>
        <button onclick="toggleRawModal(false)">&times; Close</button>
      </div>
      <div class="modal-body">
        <pre><code id="rawCode"></code></pre>
      </div>
    </div>
  </div>

  <div class="toast" id="toast">Copied to clipboard!</div>

  <script>
    const dags = {{ .DAGsJSON }} || [];
    let currentOrientation = "{{ .DefaultOrientation }}";
    let currentIndex = 0;

    // Pan & Zoom State
    let scale = 1;
    let panX = 0;
    let panY = 0;
    let isDragging = false;
    let dragStartX = 0;
    let dragStartY = 0;
    let initialPanX = 0;
    let initialPanY = 0;

    mermaid.initialize({
      startOnLoad: false,
      theme: 'neutral',
      securityLevel: 'loose',
      flowchart: {
        useMaxWidth: false,
        htmlLabels: true
      }
    });

    function updateTransform() {
      const stage = document.getElementById('diagram-stage');
      if (stage) {
        stage.style.transform = "translate(" + panX + "px, " + panY + "px) scale(" + scale + ")";
      }
      const zoomIndicator = document.getElementById('zoomLevel');
      if (zoomIndicator) {
        zoomIndicator.textContent = Math.round(scale * 100) + '%';
      }
    }

    function fitToScreen() {
      const container = document.getElementById('canvasContainer');
      const svg = document.querySelector('#diagram svg');
      if (!container || !svg) return;

      const cRect = container.getBoundingClientRect();
      let w = 0, h = 0;
      if (svg.viewBox && svg.viewBox.baseVal && svg.viewBox.baseVal.width > 0) {
        w = svg.viewBox.baseVal.width;
        h = svg.viewBox.baseVal.height;
      } else {
        const bbox = svg.getBBox();
        w = bbox.width || svg.clientWidth;
        h = bbox.height || svg.clientHeight;
      }

      if (w > 0 && h > 0 && cRect.width > 0 && cRect.height > 0) {
        const padX = 60;
        const padY = 60;
        const availW = Math.max(cRect.width - padX * 2, 80);
        const availH = Math.max(cRect.height - padY * 2, 80);
        const fitScale = Math.min(availW / w, availH / h, 1.4);
        scale = Math.max(fitScale, 0.15);
        panX = (cRect.width - w * scale) / 2;
        panY = (cRect.height - h * scale) / 2;
      } else {
        scale = 1;
        panX = 0;
        panY = 0;
      }
      updateTransform();
    }

    function resetZoom() {
      const container = document.getElementById('canvasContainer');
      const svg = document.querySelector('#diagram svg');
      scale = 1;
      if (container && svg) {
        const cRect = container.getBoundingClientRect();
        let w = 400;
        if (svg.viewBox && svg.viewBox.baseVal && svg.viewBox.baseVal.width > 0) {
          w = svg.viewBox.baseVal.width;
        }
        panX = Math.max(0, (cRect.width - w) / 2);
        panY = 40;
      } else {
        panX = 0;
        panY = 0;
      }
      updateTransform();
    }

    function zoomIn() {
      const container = document.getElementById('canvasContainer');
      const cRect = container.getBoundingClientRect();
      zoomAtPoint(cRect.width / 2, cRect.height / 2, 1.25);
    }

    function zoomOut() {
      const container = document.getElementById('canvasContainer');
      const cRect = container.getBoundingClientRect();
      zoomAtPoint(cRect.width / 2, cRect.height / 2, 0.8);
    }

    function zoomAtPoint(px, py, factor) {
      const newScale = Math.min(Math.max(scale * factor, 0.1), 5.0);
      panX = px - (px - panX) * (newScale / scale);
      panY = py - (py - panY) * (newScale / scale);
      scale = newScale;
      updateTransform();
    }

    function setupPanZoom() {
      const container = document.getElementById('canvasContainer');
      if (!container) return;

      container.addEventListener('mousedown', (e) => {
        if (e.button !== 0 || e.target.closest('.canvas-controls')) return;
        isDragging = true;
        container.classList.add('grabbing');
        dragStartX = e.clientX;
        dragStartY = e.clientY;
        initialPanX = panX;
        initialPanY = panY;
      });

      window.addEventListener('mousemove', (e) => {
        if (!isDragging) return;
        panX = initialPanX + (e.clientX - dragStartX);
        panY = initialPanY + (e.clientY - dragStartY);
        updateTransform();
      });

      window.addEventListener('mouseup', () => {
        if (isDragging) {
          isDragging = false;
          container.classList.remove('grabbing');
        }
      });

      container.addEventListener('wheel', (e) => {
        e.preventDefault();
        const rect = container.getBoundingClientRect();
        const px = e.clientX - rect.left;
        const py = e.clientY - rect.top;
        const factor = e.deltaY < 0 ? 1.15 : 0.87;
        zoomAtPoint(px, py, factor);
      }, { passive: false });

      container.addEventListener('dblclick', (e) => {
        if (e.target.closest('.canvas-controls')) return;
        fitToScreen();
      });

      window.addEventListener('keydown', (e) => {
        if (e.target.tagName === 'INPUT' || e.target.tagName === 'SELECT' || e.target.tagName === 'TEXTAREA') {
          return;
        }
        if (e.key === '+' || e.key === '=') {
          zoomIn();
        } else if (e.key === '-' || e.key === '_') {
          zoomOut();
        } else if (e.key === '0') {
          resetZoom();
        } else if (e.key === 'f' || e.key === 'F') {
          fitToScreen();
        }
      });
    }

    function init() {
      setupPanZoom();

      const selector = document.getElementById('dagSelector');
      if (!dags || dags.length === 0) {
        document.getElementById('diagram').innerHTML = '<div class="empty-state">No DAGs discovered</div>';
        return;
      }

      selector.innerHTML = '';
      dags.forEach((dag, idx) => {
        const opt = document.createElement('option');
        opt.value = idx;
        opt.textContent = '[' + idx + '] ' + dag.name + ' (' + dag.package + ') - ' + dag.location;
        selector.appendChild(opt);
      });

      const params = new URLSearchParams(window.location.search);
      const selParam = params.get('select') || params.get('id');
      if (selParam !== null) {
        const parsedIdx = parseInt(selParam, 10);
        if (!isNaN(parsedIdx) && parsedIdx >= 0 && parsedIdx < dags.length) {
          currentIndex = parsedIdx;
        } else {
          const matchIdx = dags.findIndex(d => d.id === selParam || d.name.toLowerCase() === selParam.toLowerCase());
          if (matchIdx >= 0) currentIndex = matchIdx;
        }
      }

      selector.value = currentIndex;
      renderCurrentDAG();
    }

    function onDAGSelect(idxStr) {
      currentIndex = parseInt(idxStr, 10);
      renderCurrentDAG();
    }

    function getCurrentMermaidCode() {
      if (!dags || dags.length === 0) return '';
      const dag = dags[currentIndex];
      return currentOrientation === 'LR' ? dag.mermaidLR : dag.mermaidTD;
    }

    function renderCurrentDAG() {
      if (!dags || dags.length === 0) return;
      const dag = dags[currentIndex];

      document.getElementById('metaName').textContent = dag.name;
      document.getElementById('metaPkg').textContent = dag.package;
      document.getElementById('metaLoc').textContent = dag.location;
      document.getElementById('orientLabel').textContent = currentOrientation;

      const code = getCurrentMermaidCode();
      document.getElementById('rawCode').textContent = code;
      document.getElementById('modalTitle').textContent = 'Raw Mermaid - ' + dag.name;

      const container = document.getElementById('diagram');
      container.innerHTML = '<div class="empty-state">Rendering diagram...</div>';

      const renderId = 'mermaid-' + Date.now();
      mermaid.render(renderId, code).then(result => {
        container.innerHTML = result.svg;
        const svgEl = container.querySelector('svg');
        if (svgEl) {
          svgEl.style.maxWidth = 'none';
        }
        // Smoothly fit and center diagram on canvas
        setTimeout(fitToScreen, 10);
      }).catch(err => {
        container.innerHTML = '<div style="color: #ef4444; padding: 20px;">Error rendering diagram: ' + err.message + '</div>';
      });
    }

    function toggleOrientation() {
      currentOrientation = currentOrientation === 'TD' ? 'LR' : 'TD';
      document.getElementById('orientLabel').textContent = currentOrientation;
      renderCurrentDAG();
    }

    function copyMermaid() {
      const code = getCurrentMermaidCode();
      navigator.clipboard.writeText(code).then(() => {
        showToast('Copied Mermaid syntax to clipboard!');
      }).catch(() => {
        showToast('Failed to copy');
      });
    }

    function toggleRawModal(show) {
      const modal = document.getElementById('rawModal');
      if (show) {
        modal.classList.add('active');
      } else {
        modal.classList.remove('active');
      }
    }

    function showToast(msg) {
      const toast = document.getElementById('toast');
      toast.textContent = msg;
      toast.classList.add('show');
      setTimeout(() => {
        toast.classList.remove('show');
      }, 2500);
    }

    document.addEventListener('keydown', e => {
      if (e.key === 'Escape') {
        toggleRawModal(false);
      }
    });

    window.addEventListener('DOMContentLoaded', init);
  </script>
</body>
</html>
`
