'use strict';
(()=>{
 let activeID='',timer,autoAnalyze=false;
 const fields=['host','port','user','path','fingerprint','ssh-dir','auth','password','private-key','passphrase','key-file','pattern','max-files','max-gib','recursive','auto'];
 function toggleAuth(){const key=$('remote-auth').value==='key';$('remote-password-label').hidden=key;$('remote-key-fields').hidden=!key;$('remote-password').required=!key;$('remote-private-key').required=key}
 function lock(value){for(const field of fields)$('remote-'+field).disabled=value;$('remote-submit').disabled=value;$('remote-cancel').hidden=!value;$('remote-progress').hidden=!value;importing=value;setBusy(busy);$('remote-button').disabled=busy}
 function status(text,isError=false){$('remote-status').textContent=text;$('remote-status').classList.toggle('dialog-error',isError)}
 $('remote-button').onclick=()=>{$('remote-dialog').showModal()};
 $('close-remote').onclick=()=>{$('remote-dialog').close()};
 $('remote-auth').onchange=toggleAuth;
 $('remote-key-file').onchange=async()=>{const file=$('remote-key-file').files[0];if(!file)return;if(file.size>1048576){status('私钥文件不能超过 1 MiB',true);return}try{$('remote-private-key').value=await file.text()}catch(e){status(e.message,true)}};
 $('remote-cancel').onclick=async()=>{try{await json('remote-fetch',{method:'DELETE'});status('正在取消…')}catch(e){status(e.message,true)}};
 async function watch(){
  try{
   const state=await json('remote-fetch');
   if(state.id!==activeID){lock(false);status('远程任务已变更',true);return}
   const stages={connecting:'连接中',listing:'扫描目录',downloading:'获取中',complete:'获取完成',failed:'获取失败'};
   status((stages[state.stage]||state.stage)+' · '+state.files+'/'+state.total_files+' 文件 · '+size(state.bytes)+' / '+size(state.total)+(state.current?'\n'+state.current:''));
   $('remote-progress').value=state.total?state.bytes/state.total*100:0;
   if(state.busy){timer=setTimeout(watch,350);return}
   lock(false);
   if(state.error){status(state.error==='context canceled'?'已取消':state.error,true);return}
   for(const item of state.downloads||[])if(!uploads.some(u=>u.id===item.id))uploads.push(item);
   renderFiles();
   status('已获取 '+(state.downloads||[]).length+' 个文件'+(state.warnings?.length?'\n'+state.warnings.join('\n'):''),!!state.warnings?.length);
   if(autoAnalyze){$('remote-dialog').close();await $('analyze').onclick();if(state.warnings?.length)error(Error(state.warnings.join('\n')))}
  }catch(e){status(e.message,true);timer=setTimeout(watch,1500)}
 }
 $('remote-form').onsubmit=async e=>{
  e.preventDefault();if(importing||busy)return;
  const key=$('remote-auth').value==='key';
  const options={host:$('remote-host').value.trim(),port:Number($('remote-port').value),user:$('remote-user').value.trim(),path:$('remote-path').value, fingerprint:$('remote-fingerprint').value.trim(),pattern:$('remote-pattern').value,recursive:$('remote-recursive').checked,max_files:Number($('remote-max-files').value),max_bytes:Math.floor(Number($('remote-max-gib').value)*1073741824),...(key?{private_key:$('remote-private-key').value,passphrase:$('remote-passphrase').value}:{password:$('remote-password').value})};
  autoAnalyze=$('remote-auto').checked;lock(true);status('连接中…');
  options.ssh_dir=$('remote-ssh-dir').value.trim();
  try{const result=await json('remote-fetch',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(options)});activeID=result.id;for(const field of ['password','private-key','passphrase','key-file'])$('remote-'+field).value='';clearTimeout(timer);watch()}
  catch(e){lock(false);status(e.message,true)}
 };
 toggleAuth();
})();
