// app.js — single-file SPA controller.
//
// Responsibilities, kept deliberately separated within the one file:
//   api      — fetch wrapper with auth header injection
//   router   — hash-based view dispatcher
//   views    — pure render functions returning HTML strings
//   app      — bootstrap + glue
//
// No framework; every view re-renders into #app. This keeps the UI predictable
// and matches the "minimal frontend" posture of the project.

/* ------------------------------------------------------------------ api ---- */

const API = (() => {
  const BASE = '/api/v1';
  function token() { return localStorage.getItem('token') || ''; }
  async function req(path, opts = {}) {
    const headers = { ...(opts.headers || {}) };
    if (opts.body && typeof opts.body !== 'string' && !(opts.body instanceof FormData)) {
      headers['Content-Type'] = 'application/json';
      opts.body = JSON.stringify(opts.body);
    }
    if (token()) headers['Authorization'] = 'Bearer ' + token();
    const res = await fetch(BASE + path, { ...opts, headers });
    const text = await res.text();
    let data = null;
    try { data = text ? JSON.parse(text) : null; } catch { data = text; }
    if (!res.ok) {
      const msg = (data && data.message) || ('HTTP ' + res.status);
      const err = new Error(msg); err.data = data; err.status = res.status;
      throw err;
    }
    return data;
  }
  return {
    token, setToken: (t) => t ? localStorage.setItem('token', t) : localStorage.removeItem('token'),
    get: (p) => req(p),
    post: (p, b) => req(p, { method: 'POST', body: b }),
    put: (p, b) => req(p, { method: 'PUT', body: b }),
    del: (p) => req(p, { method: 'DELETE' }),
    raw: req,
  };
})();

/* --------------------------------------------------------------- helpers ---- */

const $ = (sel, root = document) => root.querySelector(sel);
const el = (html) => { const t = document.createElement('template'); t.innerHTML = html.trim(); return t.content.firstChild; };
const esc = (s) => String(s ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const json = (v) => { try { return JSON.stringify(v, null, 2); } catch { return String(v); } };

function toast(msg, kind = '') {
  const t = el(`<div class="toast ${kind}">${esc(msg)}</div>`);
  document.body.appendChild(t);
  setTimeout(() => t.remove(), 3500);
}

function query() {
  const h = location.hash.replace(/^#\/?/, '');
  const [path, qs] = h.split('?');
  const params = {};
  if (qs) for (const pair of qs.split('&')) { const [k,v] = pair.split('='); params[k] = decodeURIComponent(v || ''); }
  return { path: path || '', params };
}
function go(p) { location.hash = p; }

/* ----------------------------------------------------------------- views --- */

const Views = {};

Views.home = () => `
  <section class="card">
    <h2>API 接口聚合与 Mock 服务平台</h2>
    <p class="muted">面向前后端开发团队的 API 协作工具：接口定义管理、智能 Mock、多接口聚合、自动文档。</p>
    <p><a href="#/projects">进入项目列表 →</a></p>
  </section>`;

Views.login = () => `
  <section class="card" style="max-width:420px;margin:0 auto">
    <h2>登录</h2>
    <form id="login-form">
      <div class="field"><label>邮箱</label><input name="email" type="email" required placeholder="you@example.com"></div>
      <div class="field"><label>密码</label><input name="password" type="password" required placeholder="≥6位"></div>
      <button type="submit">登录</button>
      <a href="#/register" class="muted" style="margin-left:10px">去注册</a>
    </form>
  </section>`;

Views.register = () => `
  <section class="card" style="max-width:420px;margin:0 auto">
    <h2>注册</h2>
    <form id="register-form">
      <div class="field"><label>用户名</label><input name="name" required></div>
      <div class="field"><label>邮箱</label><input name="email" type="email" required></div>
      <div class="field"><label>密码</label><input name="password" type="password" required placeholder="≥6位"></div>
      <button type="submit">注册</button>
      <a href="#/login" class="muted" style="margin-left:10px">去登录</a>
    </form>
  </section>`;

Views.projects = async () => {
  const q = query().params.q || '';
  const data = await API.get('/projects' + (q ? '?q=' + encodeURIComponent(q) : ''));
  const items = data.data?.items || [];
  const rows = items.map(p => `
    <tr>
      <td><a href="#/projects/${p.id}">${esc(p.name)}</a></td>
      <td class="muted">${esc(p.base_path || '—')}</td>
      <td><span class="badge ${p.visibility === 'public' ? 'published' : 'designing'}">${esc(p.visibility)}</span></td>
      <td><a href="#/projects/${p.id}/apis">接口</a> · <a href="#/projects/${p.id}/aggregates">聚合</a> · <a href="#/projects/${p.id}/docs">文档</a></td>
    </tr>`).join('');
  return `
    <section class="card">
      <div class="row"><h2 style="flex:1">项目列表</h2>
        <input id="proj-search" placeholder="搜索名称..." value="${esc(q)}">
        <a href="#/projects/new"><button class="secondary">+ 新建项目</button></a>
      </div>
      <table><thead><tr><th>名称</th><th>基础路径</th><th>可见性</th><th>操作</th></tr></thead>
      <tbody>${rows || '<tr><td colspan="4" class="muted">暂无项目</td></tr>'}</tbody></table>
    </section>`;
};

Views.projectNew = () => `
  <section class="card" style="max-width:560px">
    <h2>新建项目</h2>
    <form id="project-form">
      <div class="field"><label>名称</label><input name="name" required></div>
      <div class="field"><label>描述</label><textarea name="description" style="min-height:60px"></textarea></div>
      <div class="field"><label>基础路径</label><input name="base_path" placeholder="/api/v1"></div>
      <div class="field"><label>可见性</label><select name="visibility"><option value="private">private</option><option value="public">public</option></select></div>
      <button type="submit">创建</button>
      <a href="#/projects"><button type="button" class="secondary">取消</button></a>
    </form>
  </section>`;

Views.projectDetail = async () => {
  const id = query().path.split('/')[1];
  const [proj, stats, trend, dist] = await Promise.all([
    API.get('/projects/' + id).then(r => r.data),
    API.get('/projects/' + id + '/stats').then(r => r.data).catch(() => null),
    API.get('/projects/' + id + '/stats/trends?days=7').then(r => r.data).catch(() => null),
    API.get('/projects/' + id + '/stats/duration').then(r => r.data).catch(() => null),
  ]);
  const s = stats || {};
  const t = trend || { points: [] };
  const d = dist || { buckets: [] };
  // Trend: per-day grid with mock/aggregate counts.
  const trendCells = (t.points || []).map(p => `
    <div class="trend-day">
      <div class="date">${esc(p.date)}</div>
      <div class="counts"><span style="color:var(--accent-2)">${p.mock ?? 0}</span><span style="color:var(--accent)">${p.aggregate ?? 0}</span></div>
    </div>`).join('');
  // Latency distribution: bars per bucket.
  const maxD = Math.max(1, ...((d.buckets || []).map(b => Math.max(b.mock, b.aggregate))));
  const durRows = (d.buckets || []).map(b => {
    const mk = Math.round((b.mock / maxD) * 100), ak = Math.round((b.aggregate / maxD) * 100);
    return `<div class="chart-row">
      <span class="label">${esc(b.bucket)}</span>
      <div class="bars">
        <div class="bar mock" style="width:${mk}%"></div>
        <div class="bar aggregate" style="width:${ak}%"></div>
      </div>
      <span class="val">${b.mock}/${b.aggregate}</span>
    </div>`;
  }).join('');
  return `
    <section class="card">
      <h2>${esc(proj.name)}</h2>
      <p class="muted">${esc(proj.description || '—')}</p>
      <p class="muted">基础路径: <code>${esc(proj.base_path || '—')}</code> · 可见性: ${esc(proj.visibility)}</p>
    </section>
    <section class="card">
      <h3>统计概览</h3>
      <div class="stat-grid">
        <div class="stat"><div class="num">${s.api_count ?? '—'}</div><div class="lbl">接口总数</div></div>
        <div class="stat"><div class="num">${s.published_count ?? '—'}</div><div class="lbl">已发布</div></div>
        <div class="stat"><div class="num">${s.designing_count ?? '—'}</div><div class="lbl">设计中</div></div>
        <div class="stat"><div class="num">${s.aggregate_count ?? '—'}</div><div class="lbl">聚合接口</div></div>
        <div class="stat"><div class="num">${s.mock_call_count ?? '0'}</div><div class="lbl">Mock 调用</div></div>
        <div class="stat"><div class="num">${s.aggregate_call_count ?? '0'}</div><div class="lbl">聚合调用</div></div>
      </div>
    </section>
    <section class="card">
      <div class="row"><h3 style="flex:1">调用趋势（近7天）</h3>
        <div class="legend"><span><span class="swatch mock"></span>Mock</span><span><span class="swatch aggregate"></span>聚合</span></div>
      </div>
      <div class="trend-grid">${trendCells || '<span class="muted">暂无调用</span>'}</div>
    </section>
    <section class="card">
      <h3>响应耗时分布</h3>
      <div class="chart">${durRows || '<span class="muted">暂无数据</span>'}</div>
    </section>
    <section class="card">
      <div class="row">
        <a href="#/projects/${id}/apis"><button class="secondary">接口管理</button></a>
        <a href="#/projects/${id}/aggregates"><button class="secondary">聚合管理</button></a>
        <a href="#/projects/${id}/docs"><button class="secondary">文档预览</button></a>
        <a href="#/projects/${id}/settings"><button class="secondary">项目设置</button></a>
      </div>
    </section>`;
};

Views.apis = async () => {
  const id = query().path.split('/')[1];
  const status = query().params.status || '';
  const data = await API.get('/projects/' + id + '/apis' + (status ? '?status=' + status : ''));
  const items = data.data?.items || [];
  const rows = items.map(a => `
    <tr>
      <td><span class="badge method">${esc(a.method)}</span></td>
      <td><a href="#/projects/${id}/apis/${a.id}">${esc(a.path)}</a></td>
      <td>${esc(a.name)}</td>
      <td><span class="badge ${a.status}">${esc(a.status)}</span></td>
      <td>v${a.version}</td>
      <td><a href="#/projects/${id}/apis/${a.id}/mock">Mock</a> · <a href="#/projects/${id}/apis/${a.id}/debug">调试</a></td>
    </tr>`).join('');
  return `
    <section class="card">
      <div class="row"><h2 style="flex:1">接口列表</h2>
        <select id="api-status-filter">
          <option value="">全部状态</option>
          <option value="designing" ${status==='designing'?'selected':''}>设计中</option>
          <option value="published" ${status==='published'?'selected':''}>已发布</option>
          <option value="deprecated" ${status==='deprecated'?'selected':''}>已废弃</option>
        </select>
        <a href="#/projects/${id}/apis/new"><button class="secondary">+ 新建接口</button></a>
      </div>
      <table><thead><tr><th>方法</th><th>路径</th><th>名称</th><th>状态</th><th>版本</th><th>操作</th></tr></thead>
      <tbody>${rows || '<tr><td colspan="6" class="muted">暂无接口</td></tr>'}</tbody></table>
    </section>`;
};

Views.apiNew = async () => {
  const id = query().path.split('/')[1];
  return `
    <section class="card" style="max-width:720px">
      <h2>新建接口</h2>
      <form id="api-form">
        <div class="row">
          <div class="field col"><label>名称</label><input name="name" required></div>
          <div class="field" style="min-width:120px"><label>方法</label>
            <select name="method"><option>GET</option><option>POST</option><option>PUT</option><option>DELETE</option><option>PATCH</option></select></div>
          <div class="field col"><label>路径</label><input name="path" required placeholder="/users/:id"></div>
        </div>
        <div class="field"><label>描述</label><input name="description"></div>
        <div class="field"><label>响应 Schema (JSON Schema)</label><textarea name="response_schema" placeholder='{"type":"object","properties":{"id":{"type":"integer"}}}'></textarea></div>
        <div class="field"><label>响应示例 (JSON)</label><textarea name="response_example" placeholder='{"id":1}'></textarea></div>
        <div class="row">
          <div class="field col"><label>Mock 延迟(ms)</label><input name="mock_delay" type="number" value="0"></div>
          <div class="field col"><label>Mock 状态码</label><input name="mock_status_code" type="number" value="200"></div>
          <div class="field col"><label>标签 (逗号分隔)</label><input name="tags"></div>
        </div>
        <button type="submit">创建</button>
        <a href="#/projects/${id}/apis"><button type="button" class="secondary">取消</button></a>
      </form>
    </section>`;
};

Views.apiDetail = async () => {
  const parts = query().path.split('/');
  const pid = parts[1], aid = parts[3];
  const a = await API.get('/apis/' + aid).then(r => r.data);
  return `
    <section class="card">
      <div class="row">
        <h2 style="flex:1">${esc(a.name)}</h2>
        <span class="badge method">${esc(a.method)}</span>
        <span class="badge ${a.status}">${esc(a.status)}</span>
        <span class="muted">v${a.version}</span>
      </div>
      <p class="muted">${esc(a.description || '—')}</p>
      <p><code>${esc(a.path)}</code></p>
      <div class="row">
        <button id="publish-btn" class="secondary">发布 (版本+1)</button>
        <a href="#/projects/${pid}/apis/${aid}/mock"><button class="secondary">Mock 预览</button></a>
        <a href="#/projects/${pid}/apis/${aid}/debug"><button class="secondary">在线调试</button></a>
        <a href="#/projects/${pid}/apis/${aid}/versions"><button class="secondary">版本历史</button></a>
      </div>
    </section>
    <section class="card">
      <h3>响应 Schema</h3>
      <pre class="json">${esc(json(a.response_schema))}</pre>
    </section>
    <section class="card">
      <h3>响应示例</h3>
      <pre class="json">${esc(json(a.response_example))}</pre>
    </section>
    <section class="card">
      <h3>请求 Schema</h3>
      <pre class="json">${esc(json(a.request_schema))}</pre>
    </section>`;
};

Views.mockPreview = async () => {
  const parts = query().path.split('/');
  const pid = parts[1], aid = parts[3];
  const a = await API.get('/apis/' + aid).then(r => r.data);
  return `
    <section class="card">
      <h2>Mock 预览 · ${esc(a.name)}</h2>
      <p class="muted">调用 <code>${esc(a.method)} /mock/${pid}${esc(a.path)}</code></p>
      <button id="try-mock-btn">发起 Mock 调用</button>
    </section>
    <section class="card">
      <h3>响应</h3>
      <pre class="json" id="mock-result">(点击按钮发起调用)</pre>
    </section>`;
};

Views.debug = async () => {
  const parts = query().path.split('/');
  const pid = parts[1], aid = parts[3];
  const a = await API.get('/apis/' + aid).then(r => r.data);
  let history = [];
  try { history = (await API.get('/apis/' + aid + '/debug/history').then(r => r.data)) || []; } catch {}
  const histRows = history.slice(0, 10).map(h => `
    <tr><td class="muted">${esc(h.created_at?.slice(0,19) || '')}</td>
    <td><span class="badge method">${esc(h.request?.method)}</span> ${esc(h.request?.path || '')}</td>
    <td>${h.status_code || 0}</td><td>${h.duration || 0}ms</td></tr>`).join('');
  return `
    <section class="card">
      <h2>在线调试 · ${esc(a.name)}</h2>
      <form id="debug-form">
        <div class="row">
          <div class="field" style="min-width:120px"><label>方法</label><select name="method"><option>GET</option><option>POST</option><option>PUT</option><option>DELETE</option><option>PATCH</option></select></div>
          <div class="field col"><label>路径</label><input name="path" value="${esc(a.path)}"></div>
        </div>
        <div class="field"><label>Query (如 a=1&b=2)</label><input name="query"></div>
        <div class="field"><label>Body (JSON)</label><textarea name="body"></textarea></div>
        <button type="submit">发起调试</button>
      </form>
    </section>
    <section class="card">
      <h3>调试结果</h3>
      <pre class="json" id="debug-result">(尚未调试)</pre>
    </section>
    <section class="card">
      <h3>调试历史</h3>
      <table><thead><tr><th>时间</th><th>请求</th><th>状态码</th><th>耗时</th></tr></thead>
      <tbody>${histRows || '<tr><td colspan="4" class="muted">无记录</td></tr>'}</tbody></table>
    </section>`;
};

Views.versions = async () => {
  const parts = query().path.split('/');
  const pid = parts[1], aid = parts[3];
  const vs = (await API.get('/apis/' + aid + '/versions').then(r => r.data)) || [];
  const rows = vs.map(v => `
    <tr><td>v${v.version}</td>
    <td class="muted">${esc(v.created_at?.slice(0,19) || '')}</td>
    <td>${esc(v.change_comment || '—')}</td>
    <td><button class="secondary" data-version="${v.version}">回滚到此版本</button></td></tr>`).join('');
  return `
    <section class="card">
      <h2>版本历史</h2>
      <table><thead><tr><th>版本</th><th>时间</th><th>变更说明</th><th>操作</th></tr></thead>
      <tbody>${rows || '<tr><td colspan="4" class="muted">无版本</td></tr>'}</tbody></table>
    </section>`;
};

Views.aggregates = async () => {
  const id = query().path.split('/')[1];
  const data = await API.get('/projects/' + id + '/aggregates');
  const items = data.data?.items || [];
  const rows = items.map(a => `
    <tr><td><a href="#">${esc(a.name)}</a></td>
    <td><code>${esc(a.path)}</code></td>
    <td><span class="badge method">${esc(a.mode)}</span></td>
    <td>${a.timeout}ms</td>
    <td><a href="#/projects/${id}/aggregates/new"><button class="secondary">编辑</button></a></td></tr>`).join('');
  return `
    <section class="card">
      <div class="row"><h2 style="flex:1">聚合接口</h2>
        <a href="#/projects/${id}/aggregates/new"><button class="secondary">+ 新建聚合</button></a>
      </div>
      <table><thead><tr><th>名称</th><th>路径</th><th>模式</th><th>超时</th><th>操作</th></tr></thead>
      <tbody>${rows || '<tr><td colspan="5" class="muted">暂无聚合接口</td></tr>'}</tbody></table>
    </section>`;
};

Views.aggregateNew = async () => {
  const id = query().path.split('/')[1];
  return `
    <section class="card" style="max-width:720px">
      <h2>新建聚合接口</h2>
      <form id="aggregate-form">
        <div class="field"><label>名称</label><input name="name" required></div>
        <div class="field"><label>路径</label><input name="path" required placeholder="/users-and-orders"></div>
        <div class="row">
          <div class="field" style="min-width:180px"><label>模式</label>
            <select name="mode"><option value="parallel">并行</option><option value="serial">串行</option><option value="conditional">条件</option></select></div>
          <div class="field" style="min-width:140px"><label>超时(ms)</label><input name="timeout" type="number" value="3000"></div>
        </div>
        <div class="field"><label>下游接口配置 (JSON)</label>
          <textarea name="downstream_apis" placeholder='{"downstreams":[{"name":"user","api_id":"<id>","method":"GET","url":"http://localhost:8080/mock/<pid>/users"}]}'></textarea></div>
        <div class="field"><label>字段映射 (JSON)</label>
          <textarea name="field_mappings" placeholder='{"mappings":[{"from":"user.name","to":"username"}]}'></textarea></div>
        <button type="submit">创建</button>
        <a href="#/projects/${id}/aggregates"><button type="button" class="secondary">取消</button></a>
      </form>
    </section>`;
};

Views.docs = async () => {
  const id = query().path.split('/')[1];
  let doc = null;
  try {
    const res = await fetch('/api/v1/projects/' + id + '/docs/openapi.json', { headers: { Authorization: 'Bearer ' + API.token() } });
    doc = await res.json();
  } catch (e) { return `<section class="card"><h2>OpenAPI 文档</h2><p class="muted">加载失败: ${esc(e.message)}</p></section>`; }
  const basePath = doc.servers?.[0]?.url || '';
  const paths = doc.paths || {};
  const pathKeys = Object.keys(paths);
  // Group operations by their first tag (default "default") for Swagger-style sections.
  const groups = {};
  for (const p of pathKeys) {
    const item = paths[p];
    for (const m of ['get','post','put','delete','patch','options']) {
      const op = item[m];
      if (!op) continue;
      const tag = (op.tags && op.tags[0]) || 'default';
      (groups[tag] ||= []).push({ path: p, method: m, op });
    }
  }
  const groupHtml = Object.keys(groups).sort().map(tag => {
    const rows = groups[tag].map(({ path, method, op }) => {
      const mockUrl = '/mock/' + id + (basePath ? '' : '') + path.replace(/\{([^}]+)\}/g, ':$1');
      const params = renderParams(op.parameters);
      const reqBody = renderRequestBody(op.requestBody);
      const responses = renderResponses(op.responses);
      return `
        <div class="op-row">
          <div class="op-head" data-toggle>
            <span class="badge method ${method}">${method.toUpperCase()}</span>
            <code class="op-path">${esc(path)}</code>
            <span class="muted">${esc(op.summary || '')}</span>
            <a class="try-it" href="${mockUrl}" target="_blank">try</a>
          </div>
          <div class="op-body" style="display:none">
            ${op.description ? `<p class="muted">${esc(op.description)}</p>` : ''}
            ${params}
            ${reqBody}
            ${responses}
          </div>
        </div>`;
    }).join('');
    return `<section class="card"><h3>${esc(tag)}</h3>${rows}</section>`;
  }).join('');
  return `
    <section class="card">
      <div class="row">
        <h2 style="flex:1">${esc(doc.info?.title || 'OpenAPI')} <span class="muted" style="font-weight:normal">v${esc(doc.info?.version || '1.0')}</span></h2>
        <a href="/api/v1/projects/${id}/docs/openapi.json" target="_blank"><button class="secondary">JSON</button></a>
        <a href="/api/v1/projects/${id}/docs/openapi.yaml" target="_blank"><button class="secondary">YAML</button></a>
      </div>
      ${doc.info?.description ? `<p class="muted">${esc(doc.info.description)}</p>` : ''}
      ${basePath ? `<p class="muted">Base: <code>${esc(basePath)}</code></p>` : ''}
    </section>
    ${groupHtml || '<section class="card"><p class="muted">暂无已发布接口</p></section>'}`;
};

// renderParams returns an HTML table for path/query/header parameters.
function renderParams(params) {
  if (!params || !params.length) return '';
  const rows = params.map(p => `
    <tr><td><code>${esc(p.name)}</code></td><td><span class="badge method">${esc(p.in)}</span></td>
    <td>${p.required ? '<span class="badge published">必填</span>' : ''}</td>
    <td class="muted">${esc(p.schema?.type || '')} ${esc(p.schema?.format || '')}</td></tr>`).join('');
  return `<h4>参数</h4><table class="params"><thead><tr><th>名称</th><th>位置</th><th>必填</th><th>类型</th></tr></thead><tbody>${rows}</tbody></table>`;
}

// renderRequestBody returns an HTML block for the request body schema.
function renderRequestBody(rb) {
  if (!rb || !rb.content) return '';
  const mt = rb.content['application/json'];
  if (!mt) return '';
  return `<h4>请求体 ${rb.required ? '<span class="badge published">必填</span>' : ''}</h4>
    <pre class="json">${esc(json(mt.schema))}</pre>`;
}

// renderResponses returns an HTML block for each response status code.
function renderResponses(responses) {
  if (!responses) return '';
  const blocks = Object.keys(responses).map(code => {
    const r = responses[code];
    const ex = r.content?.['application/json']?.example;
    const sch = r.content?.['application/json']?.schema;
    return `<div class="resp"><span class="badge ${code.startsWith('2') ? 'published' : 'deprecated'}">${esc(code)}</span>
      <span class="muted">${esc(r.description || '')}</span>
      ${sch ? `<pre class="json small">${esc(json(sch))}</pre>` : ''}
      ${ex ? `<pre class="json small">${esc(json(ex))}</pre>` : ''}</div>`;
  }).join('');
  return `<h4>响应</h4>${blocks}`;
}

Views.settings = async () => {
  const id = query().path.split('/')[1];
  const [proj, members] = await Promise.all([
    API.get('/projects/' + id).then(r => r.data),
    API.get('/projects/' + id + '/members').then(r => r.data || []).catch(() => []),
  ]);
  const rows = members.map(m => `
    <tr><td>${esc(m.user_id)}</td><td><span class="badge method">${esc(m.role)}</span></td>
    <td class="muted">${esc(m.created_at?.slice(0,19) || '')}</td>
    <td><button class="danger remove-member" data-uid="${m.user_id}">移除</button></td></tr>`).join('');
  return `
    <section class="card">
      <h2>项目设置 · ${esc(proj.name)}</h2>
      <form id="project-edit-form">
        <div class="field"><label>名称</label><input name="name" value="${esc(proj.name)}"></div>
        <div class="field"><label>描述</label><textarea name="description">${esc(proj.description || '')}</textarea></div>
        <div class="field"><label>基础路径</label><input name="base_path" value="${esc(proj.base_path || '')}"></div>
        <div class="field"><label>可见性</label><select name="visibility"><option value="private" ${proj.visibility==='private'?'selected':''}>private</option><option value="public" ${proj.visibility==='public'?'selected':''}>public</option></select></div>
        <button type="submit">保存</button>
      </form>
    </section>
    <section class="card">
      <h3>成员管理</h3>
      <form id="invite-form">
        <div class="row">
          <div class="field col"><label>邮箱</label><input name="email" type="email" required></div>
          <div class="field" style="min-width:140px"><label>角色</label><select name="role"><option value="viewer">只读</option><option value="editor">编辑</option><option value="admin">管理员</option></select></div>
          <div class="field" style="margin-top:22px"><button type="submit">邀请</button></div>
        </div>
      </form>
      <table><thead><tr><th>用户ID</th><th>角色</th><th>加入时间</th><th>操作</th></tr></thead>
      <tbody>${rows || '<tr><td colspan="4" class="muted">暂无成员</td></tr>'}</tbody></table>
    </section>`;
};

/* ---------------------------------------------------------------- bindings -- */

const Bindings = {};

Bindings['login-form'] = async (f) => {
  const r = await API.post('/auth/login', Object.fromEntries(new FormData(f)));
  API.setToken(r.data.token);
  toast('登录成功');
  go('/projects');
};
Bindings['register-form'] = async (f) => {
  await API.post('/auth/register', Object.fromEntries(new FormData(f)));
  toast('注册成功，请登录');
  go('/login');
};
Bindings['project-form'] = async (f) => {
  const r = await API.post('/projects', Object.fromEntries(new FormData(f)));
  toast('项目已创建');
  go('/projects/' + r.data.id);
};
Bindings['api-form'] = async (f) => {
  const pid = query().path.split('/')[1];
  const fd = Object.fromEntries(new FormData(f));
  fd.response_schema = fd.response_schema ? JSON.parse(fd.response_schema) : {};
  fd.response_example = fd.response_example ? JSON.parse(fd.response_example) : {};
  fd.tags = fd.tags ? fd.tags.split(',').map(s => s.trim()).filter(Boolean) : [];
  fd.mock_delay = parseInt(fd.mock_delay) || 0;
  fd.mock_status_code = parseInt(fd.mock_status_code) || 200;
  const r = await API.post('/projects/' + pid + '/apis', fd);
  toast('接口已创建');
  go(`/projects/${pid}/apis/${r.data.id}`);
};
Bindings['aggregate-form'] = async (f) => {
  const pid = query().path.split('/')[1];
  const fd = Object.fromEntries(new FormData(f));
  fd.timeout = parseInt(fd.timeout) || 3000;
  fd.downstream_apis = fd.downstream_apis ? JSON.parse(fd.downstream_apis) : {};
  fd.field_mappings = fd.field_mappings ? JSON.parse(fd.field_mappings) : {};
  const r = await API.post('/projects/' + pid + '/aggregates', fd);
  toast('聚合接口已创建');
  go(`/projects/${pid}/aggregates`);
};
Bindings['debug-form'] = async (f) => {
  const aid = query().path.split('/')[3];
  const fd = Object.fromEntries(new FormData(f));
  let body = null;
  try { body = fd.body ? JSON.parse(fd.body) : {}; } catch { toast('Body JSON 解析失败', 'error'); return; }
  const r = await API.post('/apis/' + aid + '/debug', { method: fd.method, path: fd.path, query: fd.query, body });
  $('#debug-result').textContent = json(r.data);
};
Bindings['project-edit-form'] = async (f) => {
  const id = query().path.split('/')[1];
  const fd = Object.fromEntries(new FormData(f));
  await API.put('/projects/' + id, fd);
  toast('已保存');
  render();
};
Bindings['invite-form'] = async (f) => {
  const id = query().path.split('/')[1];
  await API.post('/projects/' + id + '/members', Object.fromEntries(new FormData(f)));
  toast('成员已邀请');
  render();
};

/* ----------------------------------------------------------------- router --- */

const ROUTES = [
  ['', Views.home],
  ['login', Views.login],
  ['register', Views.register],
  ['projects', Views.projects],
  ['projects/new', Views.projectNew],
  ['projects/?[^/]+/?$', Views.projectDetail],
  ['projects/?[^/]+/apis', Views.apis],
  ['projects/?[^/]+/apis/new', Views.apiNew],
  ['projects/?[^/]+/apis/?[^/]+/?$', Views.apiDetail],
  ['projects/?[^/]+/apis/?[^/]+/mock', Views.mockPreview],
  ['projects/?[^/]+/apis/?[^/]+/debug', Views.debug],
  ['projects/?[^/]+/apis/?[^/]+/versions', Views.versions],
  ['projects/?[^/]+/aggregates', Views.aggregates],
  ['projects/?[^/]+/aggregates/new', Views.aggregateNew],
  ['projects/?[^/]+/docs', Views.docs],
  ['projects/?[^/]+/settings', Views.settings],
];

async function render() {
  const { path } = query();
  const app = $('#app');
  // auth-aware redirects
  const logged = !!API.token();
  if (!logged && path !== 'login' && path !== 'register' && path !== '') {
    go('/login'); return;
  }
  if (logged && (path === 'login' || path === 'register')) {
    go('/projects'); return;
  }
  updateNav(logged);
  try {
    for (const [pat, view] of ROUTES) {
      const re = new RegExp('^' + pat + '$');
      if (re.test(path || '')) { app.innerHTML = await view(); bind(); return; }
    }
    app.innerHTML = '<section class="card"><h2>404</h2><p class="muted">页面不存在</p></section>';
  } catch (e) {
    if (e.status === 401) { API.setToken(null); go('/login'); return; }
    app.innerHTML = `<section class="card"><h2>出错了</h2><pre class="json">${esc(e.message)}</pre></section>`;
    toast(e.message, 'error');
  }
}

function updateNav(logged) {
  const nav = $('#nav');
  const auth = $('#auth-area');
  if (logged) {
    nav.innerHTML = '<a href="#/projects">项目</a>';
    auth.innerHTML = '<button id="logout-btn" class="secondary">退出</button>';
    $('#logout-btn').onclick = () => { API.setToken(null); go('/login'); };
  } else {
    nav.innerHTML = '<a href="#/login">登录</a><a href="#/register">注册</a>';
    auth.innerHTML = '';
  }
}

function bind() {
  // forms
  for (const f of document.querySelectorAll('form[id]')) {
    const handler = Bindings[f.id];
    if (handler) f.addEventListener('submit', async (e) => { e.preventDefault(); try { await handler(f); } catch (err) { toast(err.message, 'error'); } });
  }
  // publish button
  const pb = $('#publish-btn');
  if (pb) pb.onclick = async () => {
    const aid = query().path.split('/')[3];
    try { await API.post('/apis/' + aid + '/publish?comment=manual'); toast('已发布'); render(); } catch (e) { toast(e.message, 'error'); }
  };
  // try mock button
  const tm = $('#try-mock-btn');
  if (tm) tm.onclick = async () => {
    const parts = query().path.split('/'); const pid = parts[1], aid = parts[3];
    const a = await API.get('/apis/' + aid).then(r => r.data);
    const url = '/mock/' + pid + a.path;
    try { const r = await fetch(url); const t = await r.text(); let v; try { v = JSON.stringify(JSON.parse(t), null, 2); } catch { v = t; } $('#mock-result').textContent = v; }
    catch (e) { $('#mock-result').textContent = '调用失败: ' + e.message; }
  };
  // rollback buttons
  for (const b of document.querySelectorAll('button[data-version]')) {
    b.onclick = async () => {
      const aid = query().path.split('/')[3];
      try { await API.post('/apis/' + aid + '/rollback/' + b.dataset.version); toast('已回滚'); render(); } catch (e) { toast(e.message, 'error'); }
    };
  }
  // remove member
  for (const b of document.querySelectorAll('.remove-member')) {
    b.onclick = async () => {
      const id = query().path.split('/')[1];
      try { await API.del('/projects/' + id + '/members/' + b.dataset.uid); toast('已移除'); render(); } catch (e) { toast(e.message, 'error'); }
    };
  }
  // status filter
  const sf = $('#api-status-filter');
  if (sf) sf.onchange = () => { const id = query().path.split('/')[1]; go(`/projects/${id}/apis?status=${sf.value}`); };
  // project search
  const ps = $('#proj-search');
  if (ps) ps.oninput = () => { go('/projects' + (ps.value ? '?q=' + encodeURIComponent(ps.value) : '')); };
  // docs: collapsible operation rows
  for (const head of document.querySelectorAll('.op-head[data-toggle]')) {
    head.onclick = () => {
      const body = head.nextElementSibling;
      if (body) body.style.display = body.style.display === 'none' ? 'block' : 'none';
    };
  }
}

/* ---------------------------------------------------------------- bootstrap */

window.addEventListener('hashchange', render);
window.addEventListener('DOMContentLoaded', render);
