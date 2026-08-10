package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
)

type HTTPServiceStatus struct {
	Running bool     `json:"running"`
	Port    int      `json:"port"`
	URL     string   `json:"url"`
	Config  AIConfig `json:"config"`
	Error   string   `json:"error,omitempty"`
}

type HTTPService struct {
	mu         sync.Mutex
	configPath string
	client     *AIClient
	server     *http.Server
	listener   net.Listener
	port       int
	lastError  string
}

func NewHTTPService(configPath string, client *AIClient) *HTTPService {
	if client == nil {
		client = NewAIClient("", nil)
	}
	cfg, _ := LoadAIConfig(configPath)
	return &HTTPService{configPath: configPath, client: client, port: cfg.Port}
}

func (s *HTTPService) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/models", s.handleModels)
	return mux
}

func (s *HTTPService) Start(port int) error {
	cfg, err := LoadAIConfig(s.configPath)
	if err != nil {
		return err
	}
	cfg.Port = port
	cfg = NormalizeAIConfig(cfg)
	listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(cfg.Port))
	if err != nil {
		s.setError(err.Error())
		return err
	}
	server := &http.Server{Handler: s.Handler()}
	s.mu.Lock()
	if s.server != nil {
		s.mu.Unlock()
		_ = listener.Close()
		return errors.New("HTTP服务已运行")
	}
	s.server = server
	s.listener = listener
	s.port = cfg.Port
	s.lastError = ""
	s.mu.Unlock()
	if err := SaveAIConfig(s.configPath, cfg); err != nil {
		_ = s.Stop(context.Background())
		return err
	}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.setError(err.Error())
		}
	}()
	return nil
}

func (s *HTTPService) Stop(ctx context.Context) error {
	s.mu.Lock()
	server := s.server
	s.server = nil
	s.listener = nil
	s.mu.Unlock()
	if server == nil {
		return nil
	}
	return server.Shutdown(ctx)
}

func (s *HTTPService) Status() HTTPServiceStatus {
	cfg, err := LoadAIConfig(s.configPath)
	if err != nil {
		cfg = DefaultAIConfig()
	}
	s.mu.Lock()
	running := s.server != nil
	port := s.port
	lastError := s.lastError
	s.mu.Unlock()
	if port <= 0 {
		port = cfg.Port
	}
	return HTTPServiceStatus{
		Running: running,
		Port:    port,
		URL:     ConfigURL(port),
		Config:  cfg,
		Error:   lastError,
	}
}

func (s *HTTPService) setError(text string) {
	s.mu.Lock()
	s.lastError = text
	s.mu.Unlock()
}

func (s *HTTPService) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(configPageHTML))
}

func (s *HTTPService) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.Status())
}

func (s *HTTPService) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.Status().Config)
	case http.MethodPost:
		var cfg AIConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		cfg = NormalizeAIConfig(cfg)
		if err := SaveAIConfig(s.configPath, cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.mu.Lock()
		if s.server == nil {
			s.port = cfg.Port
		}
		s.mu.Unlock()
		writeJSON(w, cfg)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *HTTPService) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	models, err := s.client.ListFreeModels(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string][]string{"models": models})
}

func IsPortAvailable(port int) bool {
	if port <= 0 || port > 65535 {
		return false
	}
	listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

func ConfigURL(port int) string {
	if port <= 0 {
		port = defaultHTTPPort
	}
	return fmt.Sprintf("http://127.0.0.1:%d/", port)
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}

const configPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>AI智能聊天设置</title>
<style>
body{font-family:"Microsoft YaHei UI",Arial,sans-serif;margin:0;background:#fff;color:#202124}
main{max-width:760px;margin:32px auto;padding:0 18px}
h1{font-size:24px;margin:0 0 24px}
label{display:block;margin:14px 0 6px;font-weight:600}
input,textarea,select{box-sizing:border-box;width:100%;font:inherit;padding:8px;border:1px solid #c8cdd4;border-radius:4px}
textarea{min-height:140px;resize:vertical}
.row{display:flex;gap:16px;align-items:center;margin:14px 0}
.row label{font-weight:400;margin:0}
.row input{width:auto}
button{font:inherit;padding:8px 14px;border:1px solid #8a929d;background:#f7f7f7;border-radius:4px;cursor:pointer}
button.primary{background:#1a73e8;border-color:#1a73e8;color:#fff}
#status{margin:12px 0;color:#3367d6}
</style>
</head>
<body>
<main>
<h1>AI智能聊天设置</h1>
<div id="status"></div>
<div id="message" style="margin:10px 0;font-weight:600;"></div>
<label for="model">模型名称</label>
<div class="row"><select id="models"></select><button type="button" id="loadModels">获取可用模型列表</button></div>
<input id="model" placeholder="例如 xxx-free">
<label for="system">系统预设</label>
<textarea id="system"></textarea>
<label for="limit">对话上限</label>
<input id="limit" type="number" min="4">
<div class="row">
<label><input id="friend" type="checkbox"> 好友</label>
<label><input id="group" type="checkbox"> 群聊</label>
<label><input id="channel" type="checkbox"> 频道消息</label>
</div>
<button class="primary" type="button" id="save">保存配置</button>
</main>
<script>
function showMessage(text){message.textContent=text;}
async function loadStatus(doneText){
 try{
  const r=await fetch('/api/status');
  const s=await r.json();
  const c=s.config;
  status.textContent='HTTP服务：'+(s.running?'运行中':'未启动')+'；访问地址：'+s.url;
  model.value=c.model||'';
  system.value=c.system_prompt||'';
  limit.value=c.conversation_limit||42;
  friend.checked=!!c.enable_friend;
  group.checked=!!c.enable_group;
  channel.checked=!!c.enable_channel;
  showMessage(doneText||'配置已加载');
 }catch(e){
  showMessage('读取配置失败：'+e.message);
 }
}
loadModels.onclick=async()=>{
 try{
  showMessage('正在获取模型列表...');
  models.innerHTML='';
  const r=await fetch('/api/models');
  if(!r.ok) throw new Error(await r.text());
  const data=await r.json();
  (data.models||[]).forEach(v=>{const o=document.createElement('option');o.value=v;o.textContent=v;models.appendChild(o)});
  if(models.value) model.value=models.value;
  showMessage((data.models||[]).length?'模型列表已更新':'未获取到可用 free 模型');
 }catch(e){
  const text='获取模型列表失败：'+e.message;
  showMessage(text);
  alert(text);
 }
};
models.onchange=()=>{model.value=models.value};
save.onclick=async()=>{
 try{
  const payload={port:0,model:model.value,system_prompt:system.value,conversation_limit:Number(limit.value),enable_friend:friend.checked,enable_group:group.checked,enable_channel:channel.checked};
  const s=await (await fetch('/api/status')).json();
  payload.port=s.config.port;
  const r=await fetch('/api/config',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(payload)});
  if(!r.ok) throw new Error(await r.text());
  const text='配置已保存';
  showMessage(text);
  alert(text);
  await loadStatus(text);
 }catch(e){
  const text='保存失败：'+e.message;
  showMessage(text);
  alert(text);
 }
};
loadStatus();
</script>
</body>
</html>`
