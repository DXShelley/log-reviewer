'use strict';

function timeBound(value,end){
 const normalized=value.trim().replace(' ','T');
 const dateOnly=/^\d{4}-\d{2}-\d{2}$/.test(normalized);
 const parsed=new Date(dateOnly?normalized+'T00:00:00':normalized);
 if(!Number.isFinite(parsed.getTime()))throw Error('日期时间无效');
 let expanded=false;
 if(end&&dateOnly){parsed.setDate(parsed.getDate()+1);parsed.setMilliseconds(-1);expanded=true}
 else if(end&&!/\.\d+/.test(normalized)){if(normalized.split('T')[1].split(':').length===2)parsed.setSeconds(59);parsed.setMilliseconds(999);expanded=true}
 const iso=parsed.toISOString();return expanded?iso.replace('.999Z','.999999999Z'):iso;
}

(()=>{
 const el=id=>document.getElementById(id),pad=n=>String(n).padStart(2,'0');
 let target='',year=0,month=0,chosen='';
 const dayValue=(y,m,d)=>`${y}-${pad(m+1)}-${pad(d)}`;
 function renderCalendar(){
  el('calendar-year').value=year;el('calendar-month').value=month;
  const container=el('calendar-days');container.replaceChildren();
  const first=new Date(year,month,1).getDay(),length=new Date(year,month+1,0).getDate(),now=new Date(),today=dayValue(now.getFullYear(),now.getMonth(),now.getDate());
  for(let i=0;i<first;i++)container.append(document.createElement('span'));
  for(let day=1;day<=length;day++){
   const value=dayValue(year,month,day),button=document.createElement('button');button.type='button';button.textContent=day;button.setAttribute('aria-label',value);button.setAttribute('aria-pressed',String(value===chosen));button.classList.toggle('selected',value===chosen);button.classList.toggle('today',value===today);
   button.onclick=()=>{chosen=value;el('time-selection').textContent=chosen;renderCalendar()};container.append(button);
  }
 }
 function openTime(id){
  target=id;el('time-error').hidden=true;const current=el(id).value.replace(' ','T');
  const seed=groups.find(g=>id==='from'?g.first:g.last);const initial=current?new Date(current):seed?new Date(id==='from'?seed.first:seed.last):new Date();
  year=initial.getFullYear();month=initial.getMonth();chosen=dayValue(year,month,initial.getDate());
  el('time-hour').value=initial.getHours();el('time-minute').value=initial.getMinutes();el('time-second').value=initial.getSeconds();el('time-selection').textContent=chosen;
  el('time-picker-title').textContent=id==='from'?'开始时间':'结束时间';renderCalendar();el('time-picker').showModal();
 }
 for(let i=0;i<12;i++)el('calendar-month').add(new Option(`${i+1} 月`,i));
 for(const id of ['from','to']){el(id).onclick=()=>openTime(id);el(id).onkeydown=e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();openTime(id)}}}
 const close=()=>el('time-picker').close();el('close-time').onclick=close;el('cancel-time').onclick=close;
 function moveMonth(delta){const next=new Date(year,month+delta,1);if(next.getFullYear()<1900||next.getFullYear()>9999)return;year=next.getFullYear();month=next.getMonth();renderCalendar()}
 el('month-prev').onclick=()=>moveMonth(-1);el('month-next').onclick=()=>moveMonth(1);
 el('calendar-month').onchange=()=>{month=Number(el('calendar-month').value);renderCalendar()};
 el('calendar-year').onchange=()=>{const value=Number(el('calendar-year').value);if(Number.isInteger(value)&&value>=1900&&value<=9999){year=value;renderCalendar()}else el('calendar-year').value=year};
 el('confirm-time').onclick=()=>{
  const values=['time-hour','time-minute','time-second'].map(id=>Number(el(id).value));
  if(values.some((v,i)=>!Number.isInteger(v)||v<0||v>(i===0?23:59))||['time-hour','time-minute','time-second'].some(id=>el(id).value==='')){el('time-error').textContent='请输入有效的时、分、秒';el('time-error').hidden=false;return}
  const value=chosen+'T'+values.map(pad).join(':'),other=el(target==='from'?'to':'from').value.replace(' ','T');
  if(other&&(target==='from'?new Date(value)>new Date(other):new Date(value)<new Date(other))){el('time-error').textContent='开始时间不能晚于结束时间';el('time-error').hidden=false;return}
  el(target).value=value.replace('T',' ');close();
 };
 document.querySelectorAll('[data-reset]').forEach(button=>button.onclick=()=>{
  const type=button.dataset.reset;if(type==='time'){el('from').value='';el('to').value=''}else el(type).value=type==='source'?'-1':'';
  clearError();if(report)loadGroups();
 });
 el('paste-button').onclick=()=>{el('paste-error').hidden=true;el('paste-dialog').showModal();el('paste-text').focus()};
 el('close-paste').onclick=()=>el('paste-dialog').close();el('cancel-paste').onclick=()=>el('paste-dialog').close();
 el('confirm-paste').onclick=async()=>{
  if(!el('paste-text').value.trim()){el('paste-error').textContent='请粘贴日志文本';el('paste-error').hidden=false;return}
  const name=el('paste-name').value.trim()||'pasted.log';el('confirm-paste').disabled=true;
  try{const ok=await uploadList([new File([el('paste-text').value],name,{type:'text/plain;charset=utf-8'})]);if(ok){el('paste-dialog').close();el('paste-text').value=''}else{el('paste-error').textContent=el('error').textContent;el('paste-error').hidden=false}}finally{el('confirm-paste').disabled=false}
 };
})();
