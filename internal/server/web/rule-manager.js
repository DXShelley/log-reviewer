'use strict';
(()=>{
 const el=id=>document.getElementById(id);
 const names={java:'Java',kotlin:'Kotlin',scala:'Scala',python:'Python',javascript:'JavaScript',typescript:'TypeScript',go:'Go',csharp:'C#',c:'C',cpp:'C++',rust:'Rust',php:'PHP',ruby:'Ruby',swift:'Swift',dart:'Dart',elixir:'Elixir',lua:'Lua'};
 const fields=['id','label','evidence','start','level','warning','root'];
 let entries=[],selected='',pending=false;const drafts=new Map();
 const label=r=>r.label||names[r.id]||r.id;
 const current=()=>entries.find(e=>e.rule.id===selected);
 function showError(e){el('rule-error').textContent=e.message||String(e);el('rule-error').hidden=false}
 function readForm(){const rule={};for(const field of fields){const value=el('rule-'+field).value;if(value||field==='evidence'||field==='id')rule[field]=value}return rule}
 function renderList(){
  el('language-count').textContent=entries.length+' 种语言';el('language-list').replaceChildren();
  const rows=[...entries];if(drafts.has('$new'))rows.push({rule:drafts.get('$new'),new:true});
  for(const entry of rows){const id=entry.new?'$new':entry.rule.id,rule=drafts.get(id)||entry.rule,button=node('button');button.type='button';button.classList.toggle('active',id===selected);button.setAttribute('aria-pressed',String(id===selected));button.dataset.language=id;button.append(node('span',entry.new?(rule.label||'新增语言'):label(rule)),node('span',drafts.has(id)?'未保存':entry.builtin?(entry.modified?'已修改':'内置'):'自定义','rule-tag'));button.onclick=()=>select(id);el('language-list').append(button)}
 }
 function select(id,clear=true){
  selected=id;if(clear)el('rule-error').hidden=true;const entry=current(),rule=drafts.get(id)||entry?.rule;if(!rule)return;
  for(const field of fields)el('rule-'+field).value=field==='label'?label(rule):field==='level'?(rule[field]||'ERROR'):(rule[field]||'');
  el('rule-id').readOnly=id!=='$new';el('rule-language-title').textContent=id==='$new'?'新增语言':label(rule);el('rule-status').textContent=drafts.has(id)?'未保存':entry.builtin?(entry.modified?'已保存修改':'内置规则'):'自定义规则';
  el('restore-language').textContent=entry?.builtin?'恢复内置':'删除语言';el('restore-language').disabled=pending||id==='$new'||entry?.builtin&&!entry.modified&&!drafts.has(id);el('save-language').disabled=pending||!drafts.has(id);renderList();
 }
 async function refresh(id=selected){const data=await json('language-rules');entries=data.languages;const key=id==='$new'||entries.some(e=>e.rule.id===id)?id:entries.find(e=>e.rule.id==='java')?.rule.id||entries[0]?.rule.id;select(key)}
 function setPending(value){pending=value;el('save-language').disabled=value;el('restore-language').disabled=value;el('add-language').disabled=value;for(const field of fields)el('rule-'+field).disabled=value;el('language-list').inert=value}
 el('rules-toggle').onclick=async()=>{el('rule-manager').showModal();try{await refresh()}catch(e){showError(e)}};
 el('close-rules').onclick=()=>el('rule-manager').close();
 el('add-language').onclick=()=>{if(!drafts.has('$new'))drafts.set('$new',{id:'',label:'',evidence:'',level:'ERROR'});select('$new');el('rule-id').focus()};
 for(const field of fields)el('rule-'+field).addEventListener('input',()=>{if(!selected)return;drafts.set(selected,readForm());el('rule-status').textContent='未保存';el('save-language').disabled=false;el('restore-language').disabled=selected==='$new';renderList()});
 el('save-language').onclick=async()=>{
  el('rule-error').hidden=true;const rule=readForm();if(!rule.id||!rule.evidence){showError(Error('请填写语言标识和语言识别规则'));return}
  if(selected==='$new'&&entries.some(e=>e.rule.id===rule.id)){showError(Error('该语言已存在，请从列表选择后编辑'));return}
  const key=selected;setPending(true);try{await json('language-rules',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(rule)});drafts.delete(key);await refresh(rule.id);toast('规则已保存')}catch(e){showError(e)}finally{setPending(false);select(selected,false)}
 };
 el('restore-language').onclick=async()=>{
  const entry=current();if(!entry)return;const id=selected;el('rule-error').hidden=true;setPending(true);
  try{await json('language-rules?id='+encodeURIComponent(id),{method:'DELETE'});drafts.delete(id);await refresh(entry.builtin?id:'');toast(entry.builtin?'已恢复内置规则':'已删除语言')}catch(e){showError(e)}finally{setPending(false);select(selected,false)}
 };
})();
