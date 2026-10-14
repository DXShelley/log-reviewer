'use strict';
(() => {
 const workspace=document.querySelector('.workspace'),overrides=new Map();
 let opened=null,fullscreenTarget=null;
 const store=(key,value)=>{try{localStorage.setItem(key,value)}catch{}};
 const restore=(key,fallback)=>{try{return localStorage.getItem(key)||fallback}catch{return fallback}};
 function decorate(button,icon,label,only=false){
  const name=label||button.textContent.trim(),graphic=logIcon(icon,name).firstChild;
  button.replaceChildren(graphic);if(!only)button.append(node('span',name));else button.classList.add('icon-button');
  button.title=label||button.title||name;button.setAttribute('aria-label',label||button.getAttribute('aria-label')||name);
 }
 for(const [id,icon,label] of [['browse','FolderOpen','选择文件'],['upload-button','Upload','导入文件'],['remote-button','Server','远程获取'],['paste-button','ClipboardPaste','粘贴文本'],['path-toggle','Terminal','输入路径'],['rules-toggle','Settings','识别规则'],['analyze','Play','开始分析'],['report','Download','导出 JSON 报告'],['copy-group','Copy','复制分类'],['export','Download','导出分类']])decorate($(id),icon,label);
 for(const button of document.querySelectorAll('[data-reset],#reset'))decorate(button,'RotateCcw',button.getAttribute('aria-label'),true);
 for(const [id,icon] of [['prev','ArrowLeft'],['next','ArrowRight'],['category-prev','ArrowLeft'],['category-next','ArrowRight']])decorate($(id),icon,$(id).title,true);
 document.querySelector('.logo').replaceChildren(LogIcons.createElement(LogIcons.FileText));
 const headerActions=document.querySelector('.header-right');headerActions.prepend($('rules-toggle'));
 const layoutGroup=node('label',undefined,'layout-control');layoutGroup.append(LogIcons.createElement(LogIcons.PanelsTopLeft),node('span','布局'));
 const layout=node('select');layout.id='layout';layout.setAttribute('aria-label','界面布局');
 for(const [value,label] of [['horizontal','左右'],['vertical','上下'],['rows','上中下'],['columns','左中右'],['top','上左右'],['dialog','详情弹框']])layout.add(new Option(label,value));
 layoutGroup.append(layout);headerActions.append(layoutGroup);
 const fileActions=document.querySelector('.file-actions'),sourceActions=node('div',undefined,'source-actions'),analysisActions=node('div',undefined,'analysis-actions');
 for(const id of ['browse','upload-button','remote-button','paste-button','path-toggle'])sourceActions.append($(id));
 analysisActions.append(document.querySelector('.check'),$('analyze'),$('cancel'));fileActions.append(sourceActions,analysisActions);
 const filterBand=node('section',undefined,'filter-band'),filterHeading=node('div',undefined,'section-label');filterHeading.append(node('h2','筛选与导出'));
 $('search').before(filterBand);filterBand.append(filterHeading,$('search'));
 const controls=node('div',undefined,'view-controls'),context=document.querySelector('.context-controls');workspace.before(controls);controls.append(context);
 const autoLabel=node('label',undefined,'auto-switch'),auto=node('input');auto.type='checkbox';auto.id='context-auto';auto.checked=restore('context-auto','true')==='true';autoLabel.append(auto,node('span','自动上下文'));
 const reset=node('button','恢复 20 / 100');reset.id='reset-context';context.prepend(autoLabel);context.append(reset);
 reset.title='恢复自动模式和默认扫描上限';
 for(const [id,fallback] of [['context-before','20'],['context-after','100']]){$(id).value=restore(id,fallback);$(id).onchange=null}
 function defaults(){
  const before=Number($('context-before').value),after=Number($('context-after').value);
  if(![$('context-before').value,$('context-after').value].every(Boolean)||![before,after].every(n=>Number.isInteger(n)&&n>=0&&n<=10000))throw Error('上下文行数应为 0–10000 的整数');
  return {before,after,auto:auto.checked};
 }
 const applyDefaults=()=>{try{defaults();for(const id of ['context-before','context-after','context-auto'])store(id,id==='context-auto'?String(auto.checked):$(id).value);overrides.clear();loadDetails()}catch(e){error(e)}};
 for(const id of ['context-before','context-after'])$(id).onchange=applyDefaults;
 auto.onchange=applyDefaults;reset.onclick=()=>{$('context-before').value=20;$('context-after').value=100;auto.checked=true;applyDefaults()};
 const mask=node('dialog');mask.id='operation-mask';mask.setAttribute('aria-label','正在处理');const maskText=node('p'),maskCancel=node('button','取消分析');maskCancel.onclick=()=>$('cancel').click();mask.append(node('div',undefined,'operation-spinner'),maskText,maskCancel);document.body.append(mask);mask.addEventListener('cancel',event=>event.preventDefault());
 let exporting=0;function updateMask(){const active=busy||exporting>0;maskText.textContent=busy?'正在分析日志…':'正在导出日志…';maskCancel.hidden=!busy;if(active&&!mask.open)mask.showModal();if(!active&&mask.open)mask.close()}
 const originalSetBusy=setBusy;setBusy=function(value){originalSetBusy(value);updateMask()};
 const originalAnalyze=$('analyze').onclick;$('analyze').onclick=async function(event){setBusy(true);try{await originalAnalyze.call(this,event)}finally{poll()}};
 const originalDownload=download;download=async function(...args){exporting++;updateMask();try{return await originalDownload(...args)}finally{exporting--;updateMask()}};
 const viewer=node('section',undefined,'event-viewer');viewer.id='event-viewer';workspace.append(viewer);
 const dialog=node('dialog');dialog.id='event-dialog';const dialogHeader=node('div',undefined,'dialog-header'),dialogBody=node('div',undefined,'event-dialog-body');
 dialogHeader.append(node('h2','日志内容'),logIcon('X','关闭日志内容',()=>dialog.close()));dialog.append(dialogHeader,dialogBody);document.body.append(dialog);
 const fullscreen=node('dialog'),fullscreenBody=node('div');fullscreen.id='fullscreen-dialog';fullscreen.append(logIcon('Minimize','退出全屏',()=>fullscreen.close()),fullscreenBody);document.body.append(fullscreen);
 fullscreen.onclose=()=>{if(fullscreenTarget){fullscreenTarget.replaceChildren(...fullscreenBody.childNodes);fullscreenTarget=null}};
 function clearViewer(){viewer.replaceChildren(node('p','选择一条事件','empty'))}
 resetEventView=()=>{if(fullscreen.open)fullscreen.close();opened=null;dialogBody.replaceChildren();clearViewer();if(dialog.open)dialog.close()};
 clearViewer();$('analyze').addEventListener('click',()=>overrides.clear());
 function setLayout(){workspace.dataset.layout=layout.value;store('lr-layout',layout.value);if(dialog.open)dialog.close();if(opened)loadDetails()}
 layout.value=restore('lr-layout','horizontal');if(!layout.value)layout.value='horizontal';layout.onchange=setLayout;setLayout();
 let depth=0;const drop=node('div','释放以导入日志文件','drop-overlay');drop.hidden=true;document.body.append(drop);
 document.addEventListener('dragenter',event=>{if(Array.from(event.dataTransfer?.types||[]).includes('Files')){depth++;drop.hidden=false;drop.textContent=busy||importing?'当前任务结束后可导入':'释放以导入日志文件'}});
 document.addEventListener('dragleave',()=>{if(--depth<=0){depth=0;drop.hidden=true}});
 document.addEventListener('drop',()=>{depth=0;drop.hidden=true});window.addEventListener('blur',()=>{depth=0;drop.hidden=true});$('upload-button').title='选择文件或将多个文件拖入窗口';
 const exportMode=node('select');exportMode.id='time-export-mode';exportMode.setAttribute('aria-label','时间片段导出方式');for(const [value,label] of [['merged','合并日志'],['files','分文件 ZIP']])exportMode.add(new Option(label,value));
 const exportButton=node('button','导出时间片段');exportButton.type='button';exportButton.id='time-export';decorate(exportButton,'Download','导出时间片段');
 const exportActions=node('div',undefined,'time-export-actions');exportActions.append(node('strong','日志导出'),exportMode,exportButton);filterHeading.querySelector('h2').textContent='日志筛选';filterBand.append(exportActions);
 exportButton.onclick=async()=>{try{if(!report)throw Error('请先分析日志');if(!$('from').value||!$('to').value)throw Error('请选择开始和结束时间');const q=params();for(const key of ['group','level','keyword','regex'])q.delete(key);q.set('mode',exportMode.value);exportButton.disabled=true;await download('time-export?'+q,exportMode.value==='files'?'log-reviewer-time.zip':'log-reviewer-time.txt')}catch(e){error(e)}finally{exportButton.disabled=false}};
 const sshLabel=node('label','工具所在机器的 .ssh 目录');sshLabel.className='remote-wide';const ssh=node('input');ssh.id='remote-ssh-dir';ssh.placeholder='留空使用当前账户的 .ssh';sshLabel.append(ssh);$('remote-fingerprint').required=false;$('remote-fingerprint').placeholder='可留空，使用 .ssh/known_hosts';$('remote-fingerprint').parentElement.after(sshLabel);
 async function showFullscreen(target){if(document.fullscreenElement){await document.exitFullscreen();return}if(target.requestFullscreen){try{await target.requestFullscreen();return}catch{}}fullscreenTarget=target;fullscreenBody.replaceChildren(...target.childNodes);fullscreen.showModal()}
 function contentView(target,detail,identity,range,refresh){
  target.replaceChildren();const heading=node('div',undefined,'content-heading'),name=node('div',undefined,'content-source');
  name.append(LogIcons.createElement(LogIcons.FileText),node('strong',basename(detail.remote?.path||detail.source)),node('span','第 '+detail.line+' 行 · '+date(detail.time)));
  const status=node('span',detail.context_mode==='auto'?'自动上下文':'自定义上下文','context-mode '+(detail.context_mode||'manual'));status.title=detail.context_reason||'按指定前后行数读取';heading.append(name,status);target.append(heading);
  const toolbar=node('div',undefined,'event-tools'),before=node('input'),after=node('input');
  for(const [input,value] of [[before,range.auto?detail.context_before:range.before],[after,range.auto?detail.context_after:range.after]]){input.type='number';input.min=0;input.max=10000;input.value=value}
  before.setAttribute('aria-label','单条前文行数');after.setAttribute('aria-label','单条后文行数');const update=node('button','查询');update.title='按当前行数自定义这条事件';
  const beforeLabel=node('label','前文 '),afterLabel=node('label','后文 ');beforeLabel.append(before);afterLabel.append(after);
  const text=[detail.before,(detail.context_text||detail.text).replace(/\r?\n$/,''),detail.after].filter(Boolean).join('\n');
  const copyButton=logIcon('Copy','复制日志内容',()=>copy(text)),full=logIcon('Maximize','全屏浏览日志内容',()=>showFullscreen(target)),wrap=logIcon('WrapText','自动换行');
  wrap.setAttribute('aria-pressed','false');wrap.onclick=()=>{const value=wrap.getAttribute('aria-pressed')!=='true';wrap.setAttribute('aria-pressed',String(value));target.querySelectorAll('.log-text').forEach(pre=>pre.classList.toggle('wrap',value))};
  toolbar.append(beforeLabel,afterLabel,update,node('span','前 '+detail.context_before+' 行 / 后 '+detail.context_after+' 行','context-count'),copyButton,wrap,full);target.append(toolbar);
  target.append(logPre(text,Math.max(1,(detail.context_line||detail.line)-detail.context_before)));
  if(detail.context_text)target.append(node('div','完整事件','log-section-title'),logPre(detail.text,detail.line));
  const match=detail.text.match(/(?:trace[_-]?id|request[_-]?id|correlation[_-]?id)["'\s:=]+([\w-]+)/i);target.append(node('p',match?'请求标识：'+match[0]:'事件定位：'+basename(detail.source)+' @ '+detail.offset,'event-id'));
  if(detail.remote?.changed)target.append(node('p','远程文件在获取期间发生变化','truncated'));if(detail.truncated||detail.context_truncated)target.append(node('p','内容超过显示上限，已截断；完整内容可导出','truncated'));
  update.onclick=()=>{const b=Number(before.value),a=Number(after.value);if(!before.value||!after.value||![b,a].every(n=>Number.isInteger(n)&&n>=0&&n<=10000)){error(Error('上下文行数应为 0–10000 的整数'));return}overrides.set(identity,{before:b,after:a,auto:false});refresh()};
 }
 loadDetails=async function(){
  if(!selected)return;const group=selected,version=++detailVersion;opened=null;dialogBody.replaceChildren();clearViewer();
  $('detail').hidden=false;$('detail-empty').hidden=true;$('detail-level').replaceChildren(badge(group.level));$('detail-title').textContent=group.title;$('impact-basis').textContent=group.impact+' · '+group.impact_basis;$('scope').textContent=group.sources.length+' 个文件 · '+fmt(group.count)+' 次';$('copy-group').disabled=false;$('export').disabled=false;$('occurrences').replaceChildren(node('p','读取事件…','empty'));$('prev').disabled=true;$('next').disabled=true;
  try{
   defaults();const q=params(group.id);q.set('offset',page*PAGE);q.set('limit',PAGE);q.set('summary','1');const data=await json('details?'+q);if(version!==detailVersion)return;
   more=data.more;$('prev').disabled=page===0;$('next').disabled=!more;$('occurrence-range').textContent=data.details.length?`${page*PAGE+1}–${page*PAGE+data.details.length} / ${fmt(group.count)} 次`:'无匹配事件';$('occurrences').replaceChildren();
   data.details.forEach((summary,index)=>{
    const box=node('details',undefined,'occurrence'),head=node('summary',undefined,'occurrence-head'),identity=summary.source+':'+summary.offset,metadata=node('div',undefined,'occurrence-meta');
    metadata.append(node('strong',basename(summary.remote?.path||summary.source)),node('span','起始行 '+summary.line+' · '+date(summary.time)));head.append(metadata,node('span',summary.key||'字节 '+summary.offset,'event-id'));head.title=summary.source+' · '+(group.root_cause||group.title);
    const content=node('div');box.append(head,content);$('occurrences').append(box);let request=0;
    async function expand(){
     const serial=++request;opened=identity;const range=overrides.get(identity)||defaults();content.replaceChildren(node('p','读取内容…','event-loading'));
     const baseTarget=['columns','rows','top'].includes(layout.value)?viewer:layout.value==='dialog'?dialogBody:content,target=fullscreen.open&&fullscreenTarget===baseTarget?fullscreenBody:baseTarget;target.replaceChildren(node('p','读取内容…','event-loading'));if(layout.value==='dialog'&&!dialog.open)dialog.showModal();
     try{
      const query=params(group.id);query.set('offset',page*PAGE+index);query.set('limit',1);query.set('before',range.before);query.set('after',range.after);query.set('context',range.auto?'auto':'manual');const result=await json('details?'+query);
      if(version!==detailVersion||serial!==request||!box.open||(['columns','rows','top','dialog'].includes(layout.value)&&opened!==identity))return;
      const detail=result.details[0];if(!detail||detail.source!==summary.source||detail.offset!==summary.offset)throw Error('事件已变化，请重新查询');if(target!==content)content.replaceChildren(node('p','已在内容面板显示','event-id'));contentView(target,detail,identity,range,expand);
     }catch(e){if(serial===request&&version===detailVersion){target.replaceChildren(node('p',e.message,'dialog-error'));error(e)}}
    }
    box.ontoggle=()=>{if(box.open)expand();else request++};
   });
  }catch(e){if(version===detailVersion)error(e)}
 };
})();
