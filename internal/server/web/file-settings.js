'use strict';
let fileSettings=null,fileSettingsRequest=null;
const initialFilePatterns=['*.log','*.log.*','*.txt','*.text','*.out','*.err','*.trace'];
function applyFileSettings(settings){
 const previous=(fileSettings?.file_patterns||initialFilePatterns).join('\n');
 fileSettings=settings;
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
 const button=logIcon('FolderOpen','文件设置',async()=>{
  try{const settings=await json('import-settings');applyFileSettings(settings);directory.value=settings.remote_directory;formats.value=settings.file_patterns.join('\n');message.textContent='';dialog.showModal()}catch(e){error(e)}
 });button.id='file-settings-open';button.classList.add('icon-button');document.querySelector('.source-actions').append(button);
 const dialog=node('dialog');dialog.id='file-settings-dialog';dialog.setAttribute('aria-labelledby','file-settings-title');
 const header=node('div',undefined,'dialog-header'),title=node('h2','文件设置');title.id='file-settings-title';
 const close=logIcon('X','关闭文件设置',()=>dialog.close());header.append(title,close);
 const form=node('form'),body=node('div',undefined,'file-settings-body');
 const directoryLabel=node('label','远程文件保存目录'),directory=node('input');directory.id='remote-save-directory';directory.required=true;directoryLabel.htmlFor=directory.id;
 const formatsLabel=node('label','默认文件识别格式'),formats=node('textarea');formats.id='file-patterns';formats.required=true;formats.rows=7;formats.spellcheck=false;formatsLabel.htmlFor=formats.id;
 const reset=node('button','恢复默认值');reset.type='button';reset.onclick=async()=>{try{const settings=await json('import-settings');directory.value=settings.default_remote_directory;formats.value=settings.default_file_patterns.join('\n')}catch(e){message.textContent=e.message}};
 const message=node('p');message.id='file-settings-status';message.setAttribute('role','status');
 const footer=node('div',undefined,'dialog-footer'),save=node('button','保存');save.id='file-settings-save';save.type='submit';save.className='primary';footer.append(reset,save);
 body.append(directoryLabel,directory,formatsLabel,formats,message);form.append(body,footer);dialog.append(header,form);document.body.append(dialog);
 let saving=false;
 dialog.addEventListener('cancel',event=>{if(saving)event.preventDefault()});
 form.onsubmit=async event=>{
  event.preventDefault();if(saving)return;
  if(busy||importing){message.textContent='请等待当前分析或远程获取完成后保存';return}
  const patterns=formats.value.split(/[\n,;]+/).map(p=>p.trim()).filter(Boolean);
  saving=true;save.disabled=true;reset.disabled=true;close.disabled=true;importing=true;setBusy(busy);$('remote-button').disabled=true;button.disabled=true;
  message.textContent='正在保存并迁移文件…';
  try{
   const result=await json('import-settings',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({remote_directory:directory.value.trim(),file_patterns:patterns})});
   for(const [previous,next]of Object.entries(result.path_changes||{}))if(sourceDisplayPaths.has(previous)){sourceDisplayPaths.set(next,sourceDisplayPaths.get(previous));sourceDisplayPaths.delete(previous)}
   for(const [previous,next]of Object.entries(result.path_changes||{}))if(pathImports.has(previous)){const item=pathImports.get(previous);pathImports.set(next,{...item,path:next});pathImports.delete(previous)}
   files=files.map(p=>result.path_changes?.[p]||p);renderFiles();
   applyFileSettings(result);dialog.close();toast(result.warning||'文件设置已保存，已有文件已迁移',!!result.warning);
  }catch(e){message.textContent=e.message}
  finally{saving=false;importing=false;setBusy(busy);save.disabled=false;reset.disabled=false;close.disabled=false;button.disabled=false}
 };
})();
