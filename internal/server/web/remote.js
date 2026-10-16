'use strict';
(()=>{
 let activeID='',timer,autoAnalyze=false,pendingOptions=null,pendingHost=null,previewID='',previewFiles=[],selectedPaths=new Set(),previewLimits=null,hosts=[],pollFailures=0,sessionVersion=0,releaseRequest=Promise.resolve(),requestPromise=Promise.resolve();
 const fields=['host','port','user','path','fingerprint','ssh-dir','auth','password','private-key','passphrase','key-file','pattern','max-files','max-gib','recursive','auto'];
 const remoteAPI=(options={})=>json('remote-fetch',{signal:AbortSignal.timeout(30000),...options});
 const passwords=new Map();
 const passwordIdentity=()=>JSON.stringify([$('remote-host').value.trim(),Number($('remote-port').value),$('remote-user').value.trim()]);
 function restorePassword(){$('remote-password').value=passwords.get(passwordIdentity())||''}
 for(const field of ['host','port','user'])$('remote-'+field).addEventListener('input',restorePassword);
 for(const id of ['remote-password','remote-passphrase']){
  const input=$(id),control=node('div',undefined,'password-control');input.before(control);control.append(input);
  const toggle=logIcon('Eye','显示密码',()=>{const visible=input.type==='password';input.type=visible?'text':'password';toggle.replaceChildren(LogIcons.createElement(LogIcons[visible?'EyeOff':'Eye']));toggle.title=visible?'隐藏密码':'显示密码';toggle.setAttribute('aria-label',toggle.title);toggle.setAttribute('aria-pressed',String(visible))});toggle.setAttribute('aria-pressed','false');control.append(toggle);
 }
 const historyLabel=node('label','已连接主机','remote-history'),history=node('select');history.id='remote-history';history.setAttribute('aria-label','已连接主机');historyLabel.append(history);$('remote-form').prepend(historyLabel);
 async function loadHosts(){try{const result=await json('remote-hosts');hosts=result.hosts;history.replaceChildren(new Option('选择已连接的主机',''));hosts.forEach((host,index)=>history.add(new Option(host.user+'@'+host.host+':'+host.port,String(index))))}catch(e){status(e.message,true)}}
 history.onchange=()=>{const host=hosts[Number(history.value)];if(history.value===''||!host)return;invalidate();clearCredentials();for(const field of ['host','port','user','path','fingerprint','pattern','auth'])$('remote-'+field).value=host[field];$('remote-ssh-dir').value=host.ssh_dir||'';$('remote-recursive').checked=host.recursive;$('remote-max-files').value=host.max_files;$('remote-max-gib').value=host.max_bytes/1073741824;restorePassword();toggleAuth();status('已填入主机信息。')};
 const trust=node('div',undefined,'host-trust');trust.hidden=true;const trustText=node('p'),trustButton=node('button','已核实，使用此指纹');trustButton.type='button';trust.append(trustText,trustButton);$('remote-status').after(trust);
 trustButton.onclick=()=>{if(!pendingHost||!pendingOptions)return;const options={...pendingOptions,fingerprint:pendingHost.fingerprint};$('remote-fingerprint').value=options.fingerprint;startPreview(options)};
 const preview=node('section',undefined,'remote-preview');preview.id='remote-preview';preview.hidden=true;
 const previewHeading=node('div',undefined,'remote-preview-heading'),previewCount=node('strong'),listFilter=node('input');listFilter.type='search';listFilter.placeholder='按文件路径筛选';listFilter.setAttribute('aria-label','筛选预览文件');
 previewHeading.append(node('h3','匹配文件'),previewCount);const actions=node('div',undefined,'remote-selection-actions');actions.append(listFilter);
 function selectionButton(label,command){const button=node('button',label);button.type='button';button.onclick=command;actions.append(button)}
 selectionButton('全选',()=>{selectedPaths=new Set(previewFiles.map(file=>file.path));renderPreview()});
 selectionButton('反选',()=>{selectedPaths=new Set(previewFiles.filter(file=>!selectedPaths.has(file.path)).map(file=>file.path));renderPreview()});
 selectionButton('清空',()=>{selectedPaths.clear();renderPreview()});
 selectionButton('重置',()=>{listFilter.value='';selectedPaths=new Set(previewFiles.map(file=>file.path));renderPreview()});
 const tableWrap=node('div',undefined,'remote-preview-table'),table=node('table'),thead=node('thead'),headRow=node('tr'),tbody=node('tbody');
 for(const title of ['选择','远程路径','大小','修改时间'])headRow.append(node('th',title));thead.append(headRow);table.append(thead,tbody);tableWrap.append(table);preview.append(previewHeading,actions,tableWrap);document.querySelector('.remote-fields').after(preview);
 const downloadButton=node('button','获取所选文件');downloadButton.type='button';downloadButton.id='remote-download';downloadButton.className='primary';downloadButton.hidden=true;$('remote-submit').after(downloadButton);$('remote-submit').textContent='连接并预览';
 function selectedFiles(){return previewFiles.filter(file=>selectedPaths.has(file.path))}
 function renderPreview(){
  const chosen=selectedFiles(),bytes=chosen.reduce((sum,file)=>sum+file.size,0);previewCount.textContent='已选 '+chosen.length+' / '+previewFiles.length+' 个 · '+size(bytes)+' · 上限 '+previewLimits.max_files+' 个';
  downloadButton.disabled=!previewID||!chosen.length||chosen.length>previewLimits.max_files||bytes>previewLimits.max_bytes;
  tbody.replaceChildren();const keyword=listFilter.value.toLowerCase();
  for(const file of previewFiles){if(keyword&&!file.path.toLowerCase().includes(keyword))continue;const row=node('tr'),cell=node('td'),check=node('input');check.type='checkbox';check.checked=selectedPaths.has(file.path);check.setAttribute('aria-label',file.path);check.onchange=()=>{if(check.checked)selectedPaths.add(file.path);else selectedPaths.delete(file.path);renderPreview()};cell.append(check);row.append(cell,node('td',file.path),node('td',size(file.size)),node('td',date(file.modified)));tbody.append(row)}
 }
 listFilter.oninput=renderPreview;
 function invalidate(){previewID='';previewFiles=[];selectedPaths.clear();preview.hidden=true;downloadButton.hidden=true;pendingOptions=null;pendingHost=null;trust.hidden=true}
 for(const field of fields.filter(field=>field!=='auto'))$('remote-'+field).addEventListener('input',()=>{if(previewID||pendingHost)invalidate()});
 function clearCredentials(){for(const field of ['password','private-key','passphrase','key-file'])$('remote-'+field).value='';for(const id of ['remote-password','remote-passphrase']){const input=$(id);input.type='password';const button=input.parentElement.querySelector('button');button.replaceChildren(LogIcons.createElement(LogIcons.Eye));button.title='显示密码';button.setAttribute('aria-label','显示密码');button.setAttribute('aria-pressed','false')}}
 function toggleAuth(){const key=$('remote-auth').value==='key';$('remote-password-label').hidden=key;$('remote-key-fields').hidden=!key;$('remote-password').required=!key;$('remote-private-key').required=key}
 function lock(value){for(const field of fields)$('remote-'+field).disabled=value;history.disabled=value;$('remote-submit').disabled=value;downloadButton.disabled=value||!previewID;$('remote-cancel').hidden=!value;$('remote-progress').hidden=!value;importing=value;setBusy(busy);$('remote-button').disabled=busy;for(const button of actions.querySelectorAll('button,input'))button.disabled=value}
 function status(text,isError=false){$('remote-status').textContent=text;$('remote-status').classList.toggle('dialog-error',isError)}
 $('remote-button').onclick=async()=>{try{await ensureFileSettings();restorePassword();$('remote-dialog').showModal();loadHosts()}catch(e){error(e)}};
 $('close-remote').onclick=()=>{$('remote-dialog').close()};
 $('remote-dialog').addEventListener('close',()=>{sessionVersion++;clearTimeout(timer);activeID='';invalidate();clearCredentials();lock(false);releaseRequest=requestPromise.catch(()=>{}).then(()=>remoteAPI({method:'DELETE'})).catch(e=>error(Error('远程会话清理失败：'+e.message)))});
 $('remote-auth').onchange=toggleAuth;
 $('remote-key-file').onchange=async()=>{const file=$('remote-key-file').files[0];if(!file)return;if(file.size>1048576){status('私钥文件不能超过 1 MiB',true);return}try{$('remote-private-key').value=await file.text()}catch(e){status(e.message,true)}};
 $('remote-cancel').onclick=async()=>{pendingOptions=null;try{await remoteAPI({method:'DELETE'});status('正在取消…')}catch(e){status(e.message,true)}};
 async function watch(){
  const expectedID=activeID;if(!expectedID)return;
  try{
   const state=await remoteAPI();if(activeID!==expectedID)return;pollFailures=0;if(state.id!==activeID){lock(false);invalidate();status('远程任务已变更',true);return}
   const stages={connecting:'连接中',listing:'扫描目录',hashing:'校验内容',downloading:'获取中',complete:'获取完成',failed:'获取失败'};
   status((stages[state.stage]||state.stage)+' · '+state.files+'/'+state.total_files+' 文件 · '+size(state.bytes)+' / '+size(state.total)+(state.current?'\n'+state.current:''));$('remote-progress').value=state.total?state.bytes/state.total*100:0;
   if(state.busy){timer=setTimeout(watch,350);return}lock(false);
   if(state.error){status(state.error==='context canceled'?'已取消':state.error,true);if(state.host_key&&pendingOptions){pendingHost=state.host_key;trustText.textContent=state.host_key.host+' · '+state.host_key.fingerprint+'\n请与服务器管理员或可信控制台显示的指纹核对。';trust.hidden=false}else pendingOptions=null;return}
   pendingOptions=null;loadHosts();
   if(state.stage==='preview'){
    previewID=state.preview_id;previewFiles=state.preview_files||[];selectedPaths=new Set(previewFiles.map(file=>file.path));preview.hidden=false;downloadButton.hidden=false;listFilter.value='';renderPreview();preview.scrollIntoView({block:'nearest'});status('连接成功，匹配 '+previewFiles.length+' 个文件'+(state.warnings?.length?'\n'+state.warnings.join('\n'):''),!!state.warnings?.length);return;
   }
   previewID='';downloadButton.hidden=true;
   const added=await acceptImports(state.downloads||[]);renderFiles();status('已导入 '+added+' 个文件，跳过 '+((state.downloads||[]).length-added)+' 个文件'+(state.warnings?.length?'\n'+state.warnings.join('\n'):''),!!state.warnings?.length);
   if(autoAnalyze&&added>0){$('remote-dialog').close();await $('analyze').onclick();if(state.warnings?.length)error(Error(state.warnings.join('\n')))}
  }catch(e){if(activeID!==expectedID)return;pollFailures++;if(pollFailures>=3){activeID='';lock(false);invalidate();releaseRequest=remoteAPI({method:'DELETE'}).catch(()=>{});status('连接中断，已停止轮询，请重新连接并预览。'+e.message,true);return}status('状态查询失败，正在重试（'+pollFailures+'/3）：'+e.message,true);timer=setTimeout(watch,1500)}
 }
 async function startPreview(options){
  if(options.password)passwords.set(JSON.stringify([options.host,options.port,options.user]),options.password);
  const version=++sessionVersion;await releaseRequest;if(!$('remote-dialog').open||version!==sessionVersion)return;invalidate();pendingOptions=options;pollFailures=0;previewLimits={max_files:options.max_files,max_bytes:options.max_bytes};lock(true);status('连接中…');
  try{requestPromise=remoteAPI({method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...options,action:'preview'})});const result=await requestPromise;if(version!==sessionVersion)return;activeID=result.id;clearCredentials();restorePassword();clearTimeout(timer);watch()}
  catch(e){if(version!==sessionVersion)return;lock(false);pendingOptions=null;status(e.message,true)}
 }
 downloadButton.onclick=async()=>{
  if(importing||busy||!previewID)return;const version=sessionVersion;autoAnalyze=$('remote-auto').checked;lock(true);
  try{requestPromise=remoteAPI({method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({action:'attach',preview_id:previewID,selected_paths:selectedFiles().map(file=>file.path)})});const result=await requestPromise;if(version!==sessionVersion)return;activeID=result.id;clearTimeout(timer);watch()}
  catch(e){if(version!==sessionVersion)return;lock(false);renderPreview();status(e.message,true)}
 };
 $('remote-form').onsubmit=e=>{
  e.preventDefault();if(importing||busy)return;const key=$('remote-auth').value==='key';
  startPreview({host:$('remote-host').value.trim(),port:Number($('remote-port').value),user:$('remote-user').value.trim(),path:$('remote-path').value,fingerprint:$('remote-fingerprint').value.trim(),ssh_dir:$('remote-ssh-dir').value.trim(),pattern:$('remote-pattern').value,recursive:$('remote-recursive').checked,max_files:Number($('remote-max-files').value),max_bytes:Math.floor(Number($('remote-max-gib').value)*1073741824),...(key?{private_key:$('remote-private-key').value,passphrase:$('remote-passphrase').value}:{password:$('remote-password').value})});
 };
 toggleAuth();
})();
