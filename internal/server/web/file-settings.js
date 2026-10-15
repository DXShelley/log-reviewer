'use strict';
let fileSettings=null,fileSettingsRequest=null;
const initialFilePatterns=['*.log','*.log.*','*.txt','*.text','*.out','*.err','*.trace'];
function applyFileSettings(settings){
 const previous=(fileSettings?.file_patterns||initialFilePatterns).join('\n');
 fileSettings=settings;
	if(settings.show_file_picker!==undefined)$('upload-button').hidden=!settings.show_file_picker;
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
 $('remote-pattern-help').querySelector('p').textContent='每行一条，文件名规则任一匹配进入预览；所有远程文件（包括显式单文件）还必须符合设置中的识别格式。普通规则使用通配符，正则规则以 regex: 开头。';
 $('upload-button').title='选择文件';$('upload-button').setAttribute('aria-label','选择文件');
 const uploadText=$('upload-button').querySelector('span');if(uploadText)uploadText.textContent='选择文件';
 const button=logIcon('FolderOpen','文件设置',async()=>{
  try{const settings=await json('import-settings');applyFileSettings(settings);directory.value=settings.remote_directory;formats.value=settings.file_patterns.join('\n');message.textContent='';dialog.showModal()}catch(e){error(e)}
 });button.id='file-settings-open';button.classList.add('icon-button');button.hidden=true;
 const dialog=node('dialog');dialog.id='file-settings-dialog';dialog.setAttribute('aria-labelledby','file-settings-title');
 const header=node('div',undefined,'dialog-header'),title=node('h2','文件设置');title.id='file-settings-title';
 const close=logIcon('X','关闭文件设置',()=>dialog.close());header.append(title,close);
 const form=node('form'),body=node('div',undefined,'file-settings-body');
 const directoryLabel=node('label','远程文件保存目录'),directory=node('input');directory.id='remote-save-directory';directory.required=true;directoryLabel.htmlFor=directory.id;
 const formatsLabel=node('label','默认文件识别格式'),formats=node('textarea');formats.id='file-patterns';formats.required=true;formats.rows=7;formats.spellcheck=false;formatsLabel.htmlFor=formats.id;
 const reset=node('button','恢复默认值');reset.type='button';reset.onclick=async()=>{try{const settings=await json('import-settings');directory.value=settings.default_remote_directory;formats.value=settings.default_file_patterns.join('\n')}catch(e){message.textContent=e.message}};
 const message=node('p');message.id='file-settings-status';message.setAttribute('role','status');
 const footer=node('div',undefined,'dialog-footer'),save=node('button','保存');save.id='file-settings-save';save.type='submit';save.className='primary';footer.append(reset,save);
 const settingsDialog=$('access-dialog');
 settingsDialog.style.maxHeight='90vh';settingsDialog.style.overflowY='auto';
 body.style.padding='18px';body.style.display='grid';body.style.gap='10px';
 document.querySelector('.local').style.overflowWrap='anywhere';
 $('access-title').textContent='设置';$('access-open').title='设置';$('access-open').setAttribute('aria-label','设置');
 const accessGroup=node('h3','访问控制');settingsDialog.querySelector('.copy-body').prepend(accessGroup);
 const fileGroup=node('h3','文件与分析');
 const allLabel=$('all').closest('label');allLabel.lastChild.textContent='包含 INFO / DEBUG / TRACE';
 formatsLabel.textContent='可识别的文件格式（所有来源）';
 body.append(fileGroup,directoryLabel,directory,formatsLabel,formats,allLabel,message);form.append(body,footer);settingsDialog.append(form);
 const openSettings=$('access-open').onclick;
 $('access-open').onclick=()=>{openSettings();json('import-settings').then(settings=>{applyFileSettings(settings);directory.value=settings.remote_directory;formats.value=settings.file_patterns.join('\n')}).catch(error)};
 ensureFileSettings().catch(error);
 let saving=false;
 dialog.addEventListener('cancel',event=>{if(saving)event.preventDefault()});
 form.onsubmit=async event=>{
  event.preventDefault();if(saving)return;
  if(busy||importing){message.textContent='请等待当前分析或远程获取完成后保存';return}
  const patterns=formats.value.split(/[\n,;]+/).map(p=>p.trim()).filter(Boolean);
  saving=true;save.disabled=true;reset.disabled=true;close.disabled=true;importing=true;setBusy(busy);$('remote-button').disabled=true;button.disabled=true;
  message.textContent='正在保存并迁移文件…';
  try{
   const result=await json('import-settings',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({remote_directory:directory.value.trim(),file_patterns:patterns,include_all:$('all').checked})});
   for(const [previous,next]of Object.entries(result.path_changes||{}))if(sourceDisplayPaths.has(previous)){sourceDisplayPaths.set(next,sourceDisplayPaths.get(previous));sourceDisplayPaths.delete(previous)}
   for(const [previous,next]of Object.entries(result.path_changes||{}))if(pathImports.has(previous)){const item=pathImports.get(previous);pathImports.set(next,{...item,path:next});pathImports.delete(previous)}
   files=files.map(p=>result.path_changes?.[p]||p);renderFiles();
   applyFileSettings(result);settingsDialog.close();toast(result.warning||'文件与分析设置已保存',!!result.warning);
  }catch(e){message.textContent=e.message}
  finally{saving=false;importing=false;setBusy(busy);save.disabled=false;reset.disabled=false;close.disabled=false;button.disabled=false}
 };
})();
