'use strict';
const $=id=>document.getElementById(id);
let token=location.hash.slice(1)||sessionStorage.getItem('lr-token')||'';
if(token){sessionStorage.setItem('lr-token',token);history.replaceState(null,'',location.pathname)}
let files=[],uploads=[],report=null,groups=[],selected=null,level='',page=0,more=false,busy=false,polling=false;
let queryVersion=0,detailVersion=0,toastTimer,categoryPage=0;
const PAGE=10;
let importing=false;
const sourceDisplayPaths=new Map();
const pathImports=new Map();
const importName=p=>pathImports.get(p)?.name||basename(sourceDisplayPaths.get(p)||p);
let analysisVersion=0,pollAgain=false;
const fmt=n=>new Intl.NumberFormat('zh-CN').format(n);
const size=n=>n>=1073741824?(n/1073741824).toFixed(2)+' GiB':n>=1048576?(n/1048576).toFixed(1)+' MiB':n>=1024?(n/1024).toFixed(1)+' KiB':n+' B';
const basename=p=>p.split(/[\\/]/).pop();
const date=t=>t?new Date(t).toLocaleString('zh-CN',{hour12:false}):'时间未知';
function node(tag,text,cls){const e=document.createElement(tag);if(text!==undefined)e.textContent=text;if(cls)e.className=cls;return e}
function error(e){$('error').hidden=false;$('error').textContent=e.message||String(e);toast(e.message||String(e),true)}
function clearError(){$('error').hidden=true}
function raiseToast(){const notice=$('toast');if(notice.matches(':popover-open'))notice.hidePopover();const dialogs=[...document.querySelectorAll('dialog:modal')];(document.activeElement?.closest('dialog:modal')||dialogs.at(-1)||document.body).append(notice);notice.setAttribute('popover','manual');notice.showPopover()}
function toast(t,failed=false){clearTimeout(toastTimer);const notice=$('toast');notice.textContent=t;notice.hidden=false;notice.classList.toggle('failed',failed);notice.setAttribute('role',failed?'alert':'status');raiseToast();toastTimer=setTimeout(()=>{notice.hidePopover();notice.hidden=true},failed?7000:2400)}
document.addEventListener('close',event=>{if(event.target.contains($('toast'))&&!$('toast').hidden)raiseToast()},true);
async function api(path,opt={}){const res=await fetch('/api/'+path,{...opt,headers:{Authorization:'Bearer '+token,...opt.headers}});if(!res.ok){if(res.status===401){$('access-save').disabled=true;if(!$('access-dialog').open)$('access-dialog').showModal()}let text;try{text=(await res.json()).error}catch{text=res.statusText}throw Error(text)}return res}
async function json(path,opt){return (await api(path,opt)).json()}
function params(group){const q=new URLSearchParams();for(const id of ['keyword','regex','from','to','source']){const v=$(id).value;if(v&&v!=='-1')q.set(id,id==='from'||id==='to'?timeBound(v,id==='to'):v)}if(level)q.set('level',level);if(group)q.set('group',group);return q}
function setBusy(v){busy=v;$('remote-button').disabled=v;$('search-button').disabled=v||!report;$('analyze').disabled=v||importing||files.length+uploads.length===0;$('upload-button').disabled=v||importing;$('paste-button').disabled=v||importing;$('cancel').hidden=!v;$('progress-area').hidden=!v;const label=$('analyze').querySelector('span');if(label)label.textContent=v?'分析中…':'开始分析';else $('analyze').textContent=v?'分析中…':'开始分析';$('analyze').setAttribute('aria-label',v?'分析中…':'开始分析')}
function renderFiles(){$('files').replaceChildren();for(const [i,p]of files.entries()){const chip=node('div',undefined,'file-chip');const displayPath=sourceDisplayPaths.get(p)||p;const span=node('span',importName(p));span.title=displayPath;const b=node('button','×');b.title='移除文件';b.onclick=()=>{files.splice(i,1);renderFiles()};chip.append(span,b);$('files').append(chip)}for(const [i,u]of uploads.entries()){const chip=node('div',undefined,'file-chip');const b=node('button','×');b.title='移除文件';b.onclick=()=>{uploads.splice(i,1);renderFiles()};const name=node('span',u.name);name.title=u.remote_path||u.name;chip.append(name,b);$('files').append(chip)}$('file-count').textContent=files.length+uploads.length+' 个文件 / 路径';setBusy(busy)}
function summary(r){for(const source of r.sources)if(source.remote)sourceDisplayPaths.set(source.path,source.remote.path);if(files.length+uploads.length===0){files=r.sources.map(s=>s.path);renderFiles()}$('size-stat').textContent=size(r.bytes);$('events-stat').textContent=fmt(r.events);$('groups-stat').textContent=fmt(r.groups.length);$('error-stat').textContent=fmt(r.groups.filter(g=>['ERROR','FATAL'].includes(g.level)).reduce((n,g)=>n+g.count,0));$('warn-stat').textContent=fmt(r.groups.filter(g=>g.level==='WARN').reduce((n,g)=>n+g.count,0));$('time-stat').textContent=(r.elapsed_ms/1000).toFixed(2)+' s';$('source').replaceChildren(new Option('全部文件','-1'));r.sources.forEach((s,i)=>$('source').add(new Option(basename(s.remote?.path||s.path),i)));$('report').disabled=false;$('search-button').disabled=false;$('footer-status').textContent=r.sources.length+' 个文件 · '+fmt(r.lines)+' 行 · '+Object.keys(r.formats).join(' / ')}
async function poll(){if(polling){pollAgain=true;return}polling=true;const generation=analysisVersion;try{const state=await json('status');if(generation!==analysisVersion)return;setBusy(state.busy);if(state.busy){$('progress').value=state.total?state.bytes/state.total*100:0;$('progress-text').textContent=size(state.bytes||0)+' / '+size(state.total||0)+' · '+fmt(state.events||0)+' 事件';setTimeout(poll,350)}else{if(state.error)error(Error(state.error));if(state.ready&&report!==state.report.created+'-'+state.report.index_path){report=state.report.created+'-'+state.report.index_path;summary(state.report);selected=null;$('detail').hidden=true;resetEventView();$('detail-empty').hidden=false;await loadGroups()}}}catch(e){error(e)}finally{polling=false;if(pollAgain){pollAgain=false;setTimeout(poll,0)}}}
$('analyze').onclick=async()=>{clearError();try{const config=$('rules').value.trim()?JSON.parse($('rules').value):[];const rules=Array.isArray(config)?config:config.rules||[];const languages=Array.isArray(config)?[]:config.languages||[];await json('analyze',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({paths:files,uploads:uploads.map(u=>u.id),all:$('all').checked,rules,languages})});analysisVersion++;queryVersion++;detailVersion++;groups=[];selected=null;renderGroups();$('detail').hidden=true;resetEventView();$('detail-empty').hidden=false;$('copy-group').disabled=true;$('export').disabled=true;setBusy(true);poll()}catch(e){error(e)}};
$('cancel').onclick=async()=>{try{await json('cancel',{method:'POST'})}catch(e){error(e)}};
const pathContext=node('p',undefined,'path-context');pathContext.id='path-context';pathContext.setAttribute('role','status');$('paths').before(pathContext);$('paths').setAttribute('aria-label','工具所在机器的日志路径');
$('path-toggle').onclick=async()=>{const panel=$('paths-panel');panel.hidden=!panel.hidden;if(panel.hidden)return;$('paths').focus();try{const context=await json('paths',{signal:AbortSignal.timeout(30000)});pathContext.textContent='工具主机：'+context.hostname+' · '+context.os+' · 相对路径基准：'+context.working_directory}catch(e){error(e)}};
$('add-paths').onclick=async()=>{
 if(busy||importing)return;const requested=$('paths').value.split(/\r?\n/).map(p=>p.trim()).filter(Boolean);if(!requested.length){error(Error('请先输入文件、目录或通配符路径，每行一条'));$('paths').focus();return}
 clearError();importing=true;$('add-paths').disabled=true;$('add-paths').textContent='校验路径…';setBusy(busy);
 try{const result=await json('paths',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({paths:requested,identify:true})});const added=await acceptPaths(result.paths,result.imports);$('paths').value='';$('paths-panel').hidden=true;toast('匹配 '+result.paths.length+' 个文件，已导入 '+added+' 个');$('footer-status').textContent='路径校验完成，点击“开始分析”读取日志'}catch(e){error(e)}finally{importing=false;$('add-paths').disabled=false;$('add-paths').textContent='添加路径';setBusy(busy)}
};
function importConflict(name){return new Promise(resolve=>{
 const dialog=node('dialog'),title=node('h2','同名文件已存在'),message=node('p','是否替换分析列表中的“'+name+'”？源文件保持不变。'),label=node('label','保留文件名称'),input=node('input');
 const ext=name.lastIndexOf('.'),stem=ext>0?name.slice(0,ext):name,suffix=ext>0?name.slice(ext):'';let n=1;
 const exists=value=>uploads.some(u=>u.name===value)||files.some(p=>importName(p)===value);
 do{input.value=stem+' ('+(n++)+')'+suffix}while(exists(input.value));
 input.required=true;input.setAttribute('aria-label','保留文件名称');label.append(input);
 const actions=node('div'),overwrite=node('button','覆盖'),rename=node('button','保留并重命名'),cancel=node('button','取消');
 const finish=value=>{dialog.close();dialog.remove();resolve(value)};
 overwrite.onclick=()=>finish({overwrite:true,name});cancel.onclick=()=>finish(null);
 rename.onclick=()=>{const value=input.value.trim();input.setCustomValidity(!value||/[\\/]/.test(value)||value==='.'||value==='..'?'请输入有效文件名称':exists(value)?'名称已存在，请使用其他名称':'');if(input.reportValidity())finish({name:value})};
 input.oninput=()=>input.setCustomValidity('');dialog.oncancel=e=>{e.preventDefault();finish(null)};actions.append(overwrite,rename,cancel);dialog.append(title,message,label,actions);document.body.append(dialog);dialog.showModal();
})}
let importQueue=Promise.resolve();
async function identifyPaths(paths){
 const result=[];
 for(let i=0;i<paths.length;i+=500){const response=await json('paths',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({paths:paths.slice(i,i+500),identify:true})});result.push(...response.imports)}
 return result;
}
async function identifyExistingPaths(){
 const missing=files.filter(p=>!pathImports.has(p));
 if(missing.length)for(const item of await identifyPaths(missing))pathImports.set(item.path,item);
}
async function acceptPaths(paths,items){
 await identifyExistingPaths();
 let added=0;for(const item of items||await identifyPaths(paths))if(await acceptImport(item))added++;
 return added;
}
function acceptImport(item){const task=importQueue.then(async()=>{
 await identifyExistingPaths();
 if(uploads.some(u=>(item.id&&u.id===item.id)||(item.digest&&u.digest===item.digest))||files.some(p=>item.digest&&pathImports.get(p)?.digest===item.digest))return false;
 if(uploads.some(u=>u.name===item.name)||files.some(p=>p!==item.path&&importName(p)===item.name)){
  const choice=await importConflict(item.name);if(!choice)return false;
  if(choice.name!==item.name){if(!item.path)await json('upload-name?id='+encodeURIComponent(item.id)+'&name='+encodeURIComponent(choice.name),{method:'POST'});item={...item,name:choice.name}}
  if(choice.overwrite){uploads=uploads.filter(u=>u.name!==item.name);files=files.filter(p=>importName(p)!==item.name)}
 }
 if(item.path){pathImports.set(item.path,item);if(!files.includes(item.path))files.push(item.path)}else uploads.push(item);renderFiles();return true;
 });importQueue=task.catch(()=>{});return task}
async function uploadList(list){if(importing||busy)return false;const items=Array.from(list);if(!items.length)return false;clearError();importing=true;setBusy(busy);let added=0;try{await ensureFileSettings();for(const f of items){if(!matchesImportFormat(f.name))continue;$('footer-status').textContent='导入 '+f.name+'…';const u=await json('upload?name='+encodeURIComponent(f.name),{method:'POST',body:f});if(await acceptImport(u))added++} $('footer-status').textContent=added+' 个文件已导入，'+(items.length-added)+' 个文件已跳过（格式或重复）';return true}catch(e){error(e);return false}finally{importing=false;$('upload').value='';setBusy(busy)}}
async function chooseImport(kind){
 $('import-dialog').close();if(busy||importing)return;
 try{await ensureFileSettings();const context=await json('paths',{signal:AbortSignal.timeout(30000)});
  if(context.native_picker){importing=true;setBusy(busy);$('footer-status').textContent='请在本机系统窗口中选择日志';const result=await json('pick-paths?kind='+kind,{method:'POST'});const added=await acceptPaths(result.paths||[]);$('footer-status').textContent='已导入 '+added+' 个文件，跳过 '+((result.paths||[]).length-added)+' 个文件';return}
  if(context.local_access&&fileSettings?.show_path_input){$('paths-panel').hidden=false;$('paths').focus();toast('请填写本机文件或目录路径，直接只读分析');return}
  $(kind==='files'?'upload':'upload-directory').click();
 }catch(e){error(e)}finally{importing=false;setBusy(busy)}
}
$('upload-button').onclick=()=>$('import-dialog').showModal();$('import-files').onclick=()=>chooseImport('files');$('import-directory').onclick=()=>chooseImport('directory');$('close-import').onclick=()=>$('import-dialog').close();$('upload-directory').onchange=async()=>{try{await ensureFileSettings();await uploadList(Array.from($('upload-directory').files).filter(f=>matchesImportFormat(f.name)))}catch(e){error(e)}finally{$('upload-directory').value=''}};$('upload').onchange=()=>uploadList($('upload').files);
document.addEventListener('dragover',e=>{e.preventDefault()});document.addEventListener('drop',async e=>{e.preventDefault();if(busy||importing||!e.dataTransfer.files.length)return;const dropped=Array.from(e.dataTransfer.files);try{const context=await json('paths',{signal:AbortSignal.timeout(30000)});if(context.local_access){toast('浏览器不提供真实路径，请选择源文件或输入本机路径');await chooseImport('files')}else await uploadList(dropped)}catch(err){error(err)}});
function badge(l){return node('span',l,'badge '+l)}
function renderGroups(){
 $('groups').replaceChildren();$('result-count').textContent=fmt(groups.length)+' 类';$('empty').hidden=groups.length>0;
 if(!groups.length)$('empty').replaceChildren(node('strong',report?'未找到符合条件的事件':'选择日志文件开始分析'));
 const ranks={FATAL:5,ERROR:4,WARN:3,INFO:2,DEBUG:1,TRACE:0};
 const sorted=[...groups].sort((a,b)=>$('sort').value==='impact'?(ranks[b.level]-ranks[a.level]||b.count-a.count):$('sort').value==='last'?((Date.parse(b.last)||0)-(Date.parse(a.last)||0)):b.count-a.count);
 const visible=sorted.slice(categoryPage*100,(categoryPage+1)*100);$('category-pages').hidden=groups.length<=100;
 $('category-range').textContent=`${categoryPage*100+1}–${categoryPage*100+visible.length} / ${fmt(groups.length)} 类`;$('category-prev').disabled=categoryPage===0;$('category-next').disabled=(categoryPage+1)*100>=groups.length;
 for(const g of visible){
  const active=selected?.id===g.id,row=node('tr',undefined,'group-row'+(active?' selected':''));row.dataset.groupId=g.id;row.tabIndex=0;row.setAttribute('aria-selected',String(active));row.setAttribute('aria-label','分类 '+g.number+' '+g.level+' '+g.title);
  const title=node('td'),text=node('div',g.title,'group-title');text.title=g.title;const meta=node('div',undefined,'group-meta');
  meta.append(node('span','分类 '+g.number,'category-number'),badge(g.level),node('span',(g.languages||[]).join(' / ')||'语言未知'),node('span',g.format));title.append(text,meta);
  const impact=node('td',g.impact,'impact');impact.title=g.impact_basis;const scope=node('td',g.sources.length+' 文件');scope.title=g.loggers.join(', ');const times=node('td',undefined,'date');times.append(node('div',date(g.first)),node('div',date(g.last)));row.append(title,node('td',fmt(g.count)),impact,scope,times);
  const choose=()=>{selected=g;page=0;renderGroups();loadDetails()};row.onclick=choose;row.onkeydown=e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();choose()}};$('groups').append(row);
 }
}
async function loadGroups(){if(busy)return;const v=++queryVersion;clearError();$('search-button').disabled=true;$('result-count').textContent='搜索中…';try{const data=await json('groups?'+params());if(v!==queryVersion)return;groups=data.groups;categoryPage=0;selected=selected&&groups.find(g=>g.id===selected.id);renderGroups();if(selected){page=0;loadDetails()}else{detailVersion++;$('detail').hidden=true;resetEventView();$('detail-empty').hidden=false;$('copy-group').disabled=true;$('export').disabled=true}}catch(e){if(v===queryVersion)error(e)}finally{if(v===queryVersion)$('search-button').disabled=!report||busy}}
$('search').onsubmit=e=>{e.preventDefault();loadGroups()};$('reset').onclick=()=>{['keyword','regex','from','to'].forEach(id=>$(id).value='');$('source').value='-1';level='';document.querySelectorAll('[data-level]').forEach(b=>b.classList.toggle('active',b.dataset.level===''));if(report)loadGroups()};document.querySelectorAll('[data-level]').forEach(b=>b.onclick=()=>{level=b.dataset.level;document.querySelectorAll('[data-level]').forEach(x=>x.classList.toggle('active',x===b));if(report)loadGroups()});
async function copy(text){try{await navigator.clipboard.writeText(text);let permission;try{permission=await navigator.permissions.query({name:'clipboard-read'})}catch{}if(permission&&permission.state==='granted'&&await navigator.clipboard.readText()!==text)throw Error('系统剪贴板未保留写入内容');$('copy-dialog').close();toast('已复制')}catch(e){$('copy-status').textContent='自动复制不可用：'+e.message;$('copy-value').value=text;if(!$('copy-dialog').open)$('copy-dialog').showModal();$('copy-value').focus();$('copy-value').select()}}
let loadDetails,resetEventView=()=>{};
$('prev').onclick=()=>{page--;loadDetails()};$('next').onclick=()=>{page++;loadDetails()};$('copy-group').onclick=()=>{if(selected)copy(JSON.stringify(selected,null,2))};
async function download(path,name){clearError();let writable;try{if((path.startsWith('export?')||path.startsWith('time-export?'))&&window.showSaveFilePicker){const handle=await window.showSaveFilePicker({suggestedName:name});writable=await handle.createWritable()}const res=await api(path);if(writable){await res.body.pipeTo(writable);writable=null}else{if(Number(res.headers.get('Content-Length'))>256*1024*1024){throw Error('当前浏览器不支持流式保存大文件，请使用 Chrome / Edge 或 terminal 的 export 命令。')}const blob=await res.blob();const url=URL.createObjectURL(blob);const a=node('a');a.href=url;a.download=name;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000)}toast('已导出')}catch(e){if(writable)await writable.abort().catch(()=>{});if(e.name!=='AbortError')error(e)}}
$('export').onclick=()=>{if(selected)download('export?'+params(selected.id),'log-'+selected.id+'.txt')};$('report').onclick=()=>download('report','log-reviewer-report.json');
$('sort').onchange=()=>{categoryPage=0;renderGroups()};$('category-prev').onclick=()=>{categoryPage--;renderGroups()};$('category-next').onclick=()=>{categoryPage++;renderGroups()};
$('close-copy').onclick=()=>$('copy-dialog').close();$('retry-copy').onclick=()=>copy($('copy-value').value);$('save-copy').onclick=()=>{const url=URL.createObjectURL(new Blob([$ ('copy-value').value],{type:'text/plain;charset=utf-8'}));const a=node('a');a.href=url;a.download='log-reviewer-copy.txt';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000)};
$('context-before').onchange=loadDetails;
$('context-after').onchange=loadDetails;
async function loadAccess(){
 $('access-save').disabled=true;$('access-error').hidden=true;
 try{const data=await json('access-settings');$('access-ips').value=(data.allowed_client_ips||[]).join('\n');$('access-unrestricted').checked=!(data.allowed_client_ips||[]).length;$('access-peer').textContent='当前连接来源：'+data.remote_addr;$('access-save').disabled=false;syncAccess()}catch(e){$('access-error').textContent=e.message;$('access-error').hidden=false}
}
const accessStyle=document.createElement('link');accessStyle.rel='stylesheet';accessStyle.href='access.css';document.head.append(accessStyle);
$('access-open').replaceChildren(LogIcons.createElement(LogIcons.Settings));$('access-open').setAttribute('aria-label','访问控制');
function syncAccess(){$('access-ips').disabled=$('access-unrestricted').checked}
$('access-unrestricted').onchange=syncAccess;
$('access-open').onclick=()=>{$('access-dialog').showModal();loadAccess()};
$('access-close').onclick=()=>$('access-dialog').close();
$('access-auth').onclick=async()=>{const previous=token;token=$('access-token').value.trim();try{await json('status');sessionStorage.setItem('lr-token',token);$('access-token').value='';await loadAccess();poll()}catch(e){token=previous;$('access-error').textContent=e.message;$('access-error').hidden=false}};
$('access-save').onclick=async()=>{
 $('access-error').hidden=true;$('access-save').disabled=true;
 try{const values=$('access-ips').value.split(/[\n,]+/).map(x=>x.trim()).filter(Boolean);if(!$('access-unrestricted').checked&&!values.length)throw Error('请填写至少一个 IP 或 CIDR');await json('access-settings',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({allowed_client_ips:$('access-unrestricted').checked?[]:values})});toast('访问控制已保存');$('access-dialog').close()}catch(e){$('access-error').textContent=e.message;$('access-error').hidden=false}finally{$('access-save').disabled=false}
};
poll();
