package main

import "html/template"

var htmlTemplate = template.Must(template.New("report").Parse(htmlTemplateSrc))

const htmlTemplateSrc = `<!DOCTYPE html>
<html lang="en" data-theme="{{if .Light}}light{{else}}dark{{end}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>rawhttp report — {{.URL}}</title>
<style>
:root{
  --bg:#0d1117; --fg:#c9d1d9; --panel:#161b22; --border:#30363d;
  --muted:#8b949e; --accent:#58a6ff; --code-bg:#161b22;
  --ok:#3fb950; --redir:#d29922; --err:#f85149;
}
html[data-theme="light"]{
  --bg:#ffffff; --fg:#24292f; --panel:#f6f8fa; --border:#d0d7de;
  --muted:#57606a; --accent:#0969da; --code-bg:#f6f8fa;
  --ok:#1a7f37; --redir:#9a6700; --err:#cf222e;
}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--fg);font:14px/1.5 system-ui,-apple-system,Segoe UI,Roboto,sans-serif}
.wrap{max-width:1100px;margin:0 auto;padding:24px}
h1{font-size:18px;margin:0 0 4px}
.sub{color:var(--muted);font-size:13px;margin-bottom:20px;word-break:break-all}
.badge{display:inline-block;padding:1px 8px;border-radius:10px;font-size:12px;font-weight:600}
.badge.ok{background:var(--ok);color:#fff}.badge.err{background:var(--err);color:#fff}
.panel{background:var(--panel);border:1px solid var(--border);border-radius:8px;margin:16px 0;overflow:hidden}
.phead{display:flex;align-items:center;justify-content:space-between;padding:10px 14px;border-bottom:1px solid var(--border)}
.phead h2{font-size:14px;margin:0}
.startline{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:13px;padding:10px 14px;border-bottom:1px solid var(--border);word-break:break-all}
.startline .ok{color:var(--ok)}.startline .redir{color:var(--redir)}.startline .err{color:var(--err)}
table.hdr{width:100%;border-collapse:collapse;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12.5px}
table.hdr td{padding:3px 14px;vertical-align:top;border-bottom:1px solid var(--border)}
table.hdr td.k{color:var(--accent);white-space:nowrap;width:1%}
table.hdr td.v{color:var(--fg);word-break:break-all}
pre.code{margin:0;padding:14px;background:var(--code-bg);overflow:auto;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12.5px;line-height:1.45;border-top:1px solid var(--border)}
pre.raw{display:none}
button.copy{background:transparent;color:var(--accent);border:1px solid var(--border);border-radius:6px;padding:3px 10px;font-size:12px;cursor:pointer}
button.copy:hover{border-color:var(--accent)}
.btns{display:flex;gap:8px}
.stats{display:grid;grid-template-columns:repeat(auto-fill,minmax(220px,1fr));gap:6px 18px;padding:12px 14px}
.stats .k{color:var(--muted);font-size:12px}
.stats .val{font-family:ui-monospace,Menlo,monospace;font-size:12.5px}
.err-box{background:var(--panel);border:1px solid var(--err);border-radius:8px;padding:12px 14px;color:var(--err)}
.label{color:var(--muted);font-size:11px;text-transform:uppercase;letter-spacing:.04em;padding:8px 14px 0}
.note{color:var(--muted);font-size:12px;font-style:italic;padding:0 14px 8px;border-bottom:1px solid var(--border)}
</style>
</head>
<body>
<div class="wrap">
  <h1>{{.Tool}} <span class="badge {{if .Success}}ok{{else}}err{{end}}">{{if .Success}}OK{{else}}FAILED{{end}}</span></h1>
  <div class="sub">{{.URL}} &middot; {{.Timestamp}} &middot; {{.Tool}}/{{.Version}}{{if gt .Redirects 0}} &middot; {{.Redirects}} redirect(s){{end}}</div>

  {{if .Error}}<div class="err-box">{{.Error}}</div>{{end}}

  <div class="panel">
    <div class="phead"><h2>Request</h2><span class="btns">
      <button class="copy" onclick="copyRaw('raw-req',this)">Copy HTTP/1.1</button>
      {{if .Req.RawH2}}<button class="copy" onclick="copyRaw('raw-req-h2',this)">Copy HTTP/2</button>{{end}}
    </span></div>
    <div class="startline">{{.Req.StartLine}}</div>
    {{if .Req.Note}}<div class="note">{{.Req.Note}}</div>{{end}}
    {{template "headers" .Req.Headers}}
    {{if .Req.HasBody}}<div class="label">Body</div><pre class="code">{{.Req.Body}}</pre>{{end}}
    <pre class="raw" id="raw-req">{{.Req.Raw}}</pre>
    {{if .Req.RawH2}}<pre class="raw" id="raw-req-h2">{{.Req.RawH2}}</pre>{{end}}
  </div>

  {{with .Resp}}
  <div class="panel">
    <div class="phead"><h2>Response</h2><button class="copy" onclick="copyRaw('raw-resp',this)">Copy</button></div>
    <div class="startline"><span class="{{.StatusClass}}">{{.StartLine}}</span></div>
    {{template "headers" .Headers}}
    {{if .HasBody}}<div class="label">Body</div><pre class="code">{{.Body}}</pre>{{end}}
    <pre class="raw" id="raw-resp">{{.Raw}}</pre>
  </div>
  {{end}}

  {{if .Stats}}
  <div class="panel">
    <div class="phead"><h2>Connection &amp; timing</h2></div>
    <div class="stats">
      {{range .Stats}}<div><div class="k">{{.K}}</div><div class="val">{{.V}}</div></div>{{end}}
    </div>
  </div>
  {{end}}
</div>
<script>
function copyRaw(id, btn){
  var el=document.getElementById(id);
  navigator.clipboard.writeText(el.textContent).then(function(){
    var t=btn.textContent; btn.textContent='Copied'; setTimeout(function(){btn.textContent=t;},1200);
  });
}
</script>
</body>
</html>
{{define "headers"}}{{if .}}<table class="hdr">{{range .}}<tr><td class="k">{{.Name}}</td><td class="v">{{.Value}}</td></tr>{{end}}</table>{{end}}{{end}}
`
