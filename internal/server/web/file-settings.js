'use strict';
let fileSettings=null,fileSettingsRequest=null;
const initialFilePatterns=['*.log','*.log.*','*.txt','*.text','*.out','*.err','*.trace'];
function applyFileSettings(settings){
 const previous=(fileSettings?.file_patterns||initialFilePatterns).join('\n');
 fileSettings=settings;
	if(settings.show_file_picker!==undefined)$('file-picker-button').hidden=!settings.show_file_picker;
	if(settings.show_path_input!==undefined)$('path-toggle').hidden=!settings.show_path_input;
	if(settings.include_all!==undefined)$('all').checked=settings.include_all;
	if(settings.hostname){const label=document.querySelector('.local');label.textContent='分析主机：'+settings.hostname;label.title='日志由运行 Log Reviewer 的主机处理；远程文件需要传输到该主机读取，不调用云端分析服务'}
 const patterns=settings.file_patterns;
 const accept=patterns.filter(p=>/^\*\.[a-z0-9]+$/i.test(p)).map(p=>p.slice(1)).join(',');
 $('upload').accept=accept;
 if(!$('remote-pattern').value.trim()||$('remote-pattern').value.trim()===previous)$('remote-pattern').value=patterns.join('\n');
}
async function ensureFileSettings(){
 if(fileSettings)return fileSettings;
 if(!fileSettingsRequest)fileSettingsRequest=json('import-settings').then(settings=>{applyFileSettings(settings);return settings}).finally(()=>{fileSettingsRequest=null});
 return fileSettingsRequest;
}
function matchesImportFormat(name){
 return (fileSettings?.file_patterns||initialFilePatterns).some(pattern=>{
  // Directory uploads use the same filename glob semantics as the server.
  let expression='';
  for(let i=0;i<pattern.length;i++){
   const c=pattern[i];
   if(c==='*')expression+='.*';else if(c==='?')expression+='.';
   else if(c==='['){const end=pattern.indexOf(']',i+1);if(end<0)return false;expression+='['+pattern.slice(i+1,end)+']';i=end}
   else expression+=c.replace(/[\\^$.*+?()[\]{}|]/g,'\\$&');
  }
  try{return new RegExp('^'+expression+'$','is').test(name)}catch{return false}
 });
}
(()=>{
 $('path-toggle').hidden=true;
 $('path-toggle').title='选择路径';$('path-toggle').setAttribute('aria-label','选择路径');
 $('path-toggle').querySelector('span').textContent='选择路径';
 $('remote-pattern-help').querySelector('p').textContent='每行一条，文件名规则任一匹配进入预览；所有远程文件（包括显式单文件）还必须符合设置中的识别格式。普通规则使用通配符，正则规则以 regex: 开头。';
 $('upload-button').title='将浏览器本机文件上传到部署机器';$('upload-button').setAttribute('aria-label','上传文件');
 const uploadText=$('upload-button').querySelector('span');if(uploadText)uploadText.textContent='上传文件';
 const form=node('form'),body=node('div',undefined,'file-settings-body');
 const directoryLabel=node('label','远程文件保存目录'),directory=node('input');directory.id='remote-save-directory';directory.required=true;directoryLabel.htmlFor=directory.id;
 const formatsLabel=node('label','默认文件识别格式'),formats=node('textarea');formats.id='file-patterns';formats.required=true;formats.rows=7;formats.spellcheck=false;formatsLabel.htmlFor=formats.id;
 const reset=node('button','恢复默认值');reset.type='button';reset.onclick=async()=>{try{const settings=await json('import-settings');directory.value=settings.default_remote_directory;formats.value=settings.default_file_patterns.join('\n')}catch(e){message.textContent=e.message}};
 const message=node('p');message.id='file-settings-status';message.setAttribute('role','status');
 const footer=node('div',undefined,'dialog-footer'),save=node('button','保存');save.id='file-settings-save';save.type='submit';save.className='primary';footer.append(reset,save);
 const settingsDialog=$('access-dialog');
 const authenticationDialog=node('dialog');authenticationDialog.id='authentication-dialog';
 const authenticationHeader=node('div',undefined,'dialog-header'),authenticationTitle=node('h2','会话验证'),authenticationClose=node('button','×');
 authenticationTitle.id='authentication-title';authenticationDialog.setAttribute('aria-labelledby',authenticationTitle.id);
 authenticationClose.title='关闭';authenticationClose.setAttribute('aria-label','关闭会话验证');authenticationClose.onclick=()=>authenticationDialog.close();authenticationHeader.append(authenticationTitle,authenticationClose);
 const authenticationBody=node('div',undefined,'copy-body');
 authenticationBody.append(settingsDialog.querySelector('label[for="access-token"]'),$('access-token'),$('access-auth'));
 authenticationDialog.append(authenticationHeader,authenticationBody);document.body.append(authenticationDialog);
 const authenticationOpen=node('button','会话验证');authenticationOpen.id='authentication-open';authenticationOpen.title='验证会话 token';authenticationOpen.onclick=()=>authenticationDialog.showModal();
 const headerActions=document.querySelector('.header-right');headerActions.prepend(document.querySelector('.local'));document.querySelector('.local').after(authenticationOpen);
 const close=$('access-close');
 body.style.padding='18px';body.style.display='grid';body.style.gap='10px';
 document.querySelector('.local').style.overflowWrap='anywhere';
 $('access-title').textContent='设置';$('access-open').title='设置';$('access-open').setAttribute('aria-label','设置');
 const accessPanel=node('section');accessPanel.id='settings-access-panel';
 accessPanel.append(settingsDialog.querySelector('.copy-body'),settingsDialog.querySelector('.dialog-footer'));
 settingsDialog.append(accessPanel);
 form.id='settings-files-panel';
 const tabs=node('div',undefined,'settings-tabs');tabs.setAttribute('role','tablist');tabs.setAttribute('aria-label','设置分类');
 const rulesPanel=node('section');rulesPanel.id='settings-rules-panel';rulesPanel.append($('rule-manager').querySelector('.rule-manager-body'));settingsDialog.append(rulesPanel);$('rules-toggle').hidden=true;
 const panels=[accessPanel,form,rulesPanel],tabButtons=[];
 function selectSettings(index){panels.forEach((panel,i)=>{panel.hidden=i!==index;tabButtons[i].setAttribute('aria-selected',String(i===index));tabButtons[i].tabIndex=i===index?0:-1});settingsDialog.classList.toggle('show-rules',index===2);if(index===2)rulesPanel.dispatchEvent(new Event('rules-open'))}
 for(const [i,label]of ['访问控制','文件与分析','识别规则'].entries()){
  const tab=node('button',label);tab.type='button';tab.id='settings-tab-'+i;tab.setAttribute('role','tab');tab.setAttribute('aria-controls',panels[i].id);
  panels[i].setAttribute('role','tabpanel');panels[i].setAttribute('aria-labelledby',tab.id);
  tab.onclick=()=>selectSettings(i);tab.onkeydown=event=>{if(['ArrowLeft','ArrowRight','Home','End'].includes(event.key)){event.preventDefault();const next=event.key==='Home'?0:event.key==='End'?panels.length-1:(i+(event.key==='ArrowRight'?1:-1)+panels.length)%panels.length;selectSettings(next);tabButtons[next].focus()}};
  tabButtons.push(tab);tabs.append(tab);
 }
 settingsDialog.querySelector('.dialog-header').after(tabs);
 const fileGroup=node('h3','文件与分析');
 const allLabel=$('all').closest('label');allLabel.lastChild.textContent='包含 INFO / DEBUG / TRACE';
 formatsLabel.textContent='可识别的文件格式（所有来源）';
 body.append(fileGroup,directoryLabel,directory,formatsLabel,formats,allLabel,message);form.append(body,footer);settingsDialog.append(form);
 selectSettings(0);
 const openSettings=$('access-open').onclick;
 async function loadFileSettings(){
  message.textContent='';save.disabled=true;
  try{const settings=await json('import-settings');applyFileSettings(settings);directory.value=settings.remote_directory;formats.value=settings.file_patterns.join('\n');save.disabled=false}catch(e){message.textContent=e.message;error(e)}
 }
 $('access-open').onclick=()=>{selectSettings(0);openSettings();loadFileSettings()};
 const authenticate=$('access-auth').onclick;
 const authenticationError=node('p');authenticationError.setAttribute('role','alert');authenticationBody.append(authenticationError);
 $('access-auth').onclick=async()=>{await authenticate();if($('access-error').hidden){authenticationDialog.close();await loadFileSettings()}else authenticationError.textContent=$('access-error').textContent};
 ensureFileSettings().catch(error);
 let saving=false;
 settingsDialog.addEventListener('cancel',event=>{if(saving)event.preventDefault()});
 form.onsubmit=async event=>{
  event.preventDefault();if(saving)return;
  if(busy||importing){message.textContent='请等待当前分析或远程获取完成后保存';return}
  const patterns=formats.value.split(/[\n,;]+/).map(p=>p.trim()).filter(Boolean);
  saving=true;save.disabled=true;reset.disabled=true;close.disabled=true;importing=true;setBusy(busy);$('remote-button').disabled=true;tabButtons.forEach(tab=>tab.disabled=true);
  message.textContent='正在保存并迁移文件…';
  try{
   const result=await json('import-settings',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({remote_directory:directory.value.trim(),file_patterns:patterns,include_all:$('all').checked})});
   for(const [previous,next]of Object.entries(result.path_changes||{}))if(sourceDisplayPaths.has(previous)){sourceDisplayPaths.set(next,sourceDisplayPaths.get(previous));sourceDisplayPaths.delete(previous)}
   for(const [previous,next]of Object.entries(result.path_changes||{}))if(pathImports.has(previous)){const item=pathImports.get(previous);pathImports.set(next,{...item,path:next});pathImports.delete(previous)}
   files=files.map(p=>result.path_changes?.[p]||p);renderFiles();
   applyFileSettings(result);settingsDialog.close();toast(result.warning||'文件与分析设置已保存',!!result.warning);
  }catch(e){message.textContent=e.message}
  finally{saving=false;importing=false;setBusy(busy);save.disabled=false;reset.disabled=false;close.disabled=false;tabButtons.forEach(tab=>tab.disabled=false)}
 };
})();
