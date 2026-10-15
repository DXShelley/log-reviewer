'use strict';
(() => {
 const highlighter=window.LogHighlight;
 if(highlighter)highlighter.registerLanguage('log',()=>{
 const signals=[
  {scope:'severity-fatal',match:/\b(?:FATAL|CRITICAL|PANIC|SEVERE|EMERG)\b/},
  {scope:'severity-error',match:/\b(?:ERROR|ERR|Error|Exception|Caused by|Suppressed)(?:\b|:)/},
  {scope:'severity-warn',match:/\b(?:WARN|WARNING|Warning)\b/},
  {scope:'severity-info',match:/\b(?:INFO|DEBUG|TRACE|NOTICE)\b/},
  {scope:'timestamp',match:/\b\d{4}[-/]\d{2}[-/]\d{2}[T ]\d{2}:\d{2}:\d{2}(?:[.,]\d{1,9})?(?:Z|[+-]\d{2}:\d{2})?/},
  {scope:'request',match:/\b(?:trace[_-]?id|request[_-]?id|correlation[_-]?id)["'\s:=]+[\w-]+/i},
  {scope:'location',match:/(?<![\w$./\\-])(?:[A-Za-z]:[\\/]|\/)?[\w$./\\-]+\.(?:java|kt|scala|py|js|ts|go|rs|cs|cpp|c|rb|php):\d+(?::\d+)?/},
  {scope:'exception',match:/(?<![\w.$])[\w.$]*(?:Exception|Error|Panic)\b/},
  {scope:'stack',match:/^\s*(?:at\s|File\s|goroutine\s|#\d+\s|stack backtrace:)/m}
 ];
 return {name:'Log',contains:[
  {scope:'severity-error',match:/"(?:level|severity|@l)"\s*:\s*(?:[56]0\b|"(?:error|fatal|critical)")/i},
  {scope:'severity-warn',match:/"(?:level|severity|@l)"\s*:\s*(?:40\b|"(?:warn|warning)")/i},
  ...signals,{scope:'attr',match:/"[^"\r\n]{1,80}"(?=\s*:)/},
  {scope:'string',begin:/"/,end:/"/,contains:[{begin:/\\./},...signals]}
 ]};
 });
 window.logIcon=function(name,label,command){
  const button=node('button');button.type='button';button.className='icon-button';button.title=label;button.setAttribute('aria-label',label);
  if(window.LogIcons?.[name]){const svg=LogIcons.createElement(LogIcons[name]);svg.setAttribute('aria-hidden','true');button.append(svg)}else button.textContent=label;
  if(command)button.onclick=command;return button;
 };
 window.logPre=function(text,startLine=1){
  const pre=node('pre',undefined,'log-text');pre.dataset.startLine=startLine;
  const code=node('code');pre.append(code);
  // Bound coloring work; the remaining preview stays readable and copyable.
  const limit=256*1024;let end=Math.min(text.length,limit);
  if(end<text.length){const boundary=text.lastIndexOf('\n',end);if(boundary>0)end=boundary+1}
  const prefix=text.slice(0,end),canColor=highlighter&&!prefix.split('\n').some(line=>line.length>4096);
  if(canColor){try{code.innerHTML=highlighter.highlight(prefix,{language:'log',ignoreIllegals:true}).value}catch{code.textContent=prefix}}else code.textContent=prefix;
  if(end<text.length)code.append(document.createTextNode(text.slice(end)));
  pre.addEventListener('dblclick',event=>{
   const location=event.target.closest('.hljs-location');if(!location)return;
   const range=document.createRange();range.selectNodeContents(location);const selection=window.getSelection();selection.removeAllRanges();selection.addRange(range);copy(location.textContent);
  });
  return pre;
 };
 window.logSearch=function(target,header,toolbar){
  target.tabIndex=0;
  const panel=node('div',undefined,'log-search'),input=node('input'),counter=node('span'),sensitive=node('input'),regex=node('input');
  panel.hidden=true;panel.setAttribute('role','search');input.type='search';input.placeholder='查找内容';input.setAttribute('aria-label','查找日志内容');sensitive.type=regex.type='checkbox';
  const caseLabel=node('label','区分大小写'),regexLabel=node('label','正则表达式');caseLabel.prepend(sensitive);regexLabel.prepend(regex);counter.setAttribute('role','status');
  let hits=[],current=-1,marker=null,request=0,worker=null,workerTimer,searchTimer,limited=false;
  function stopWorker(){clearTimeout(workerTimer);worker?.terminate();worker=null}
  function select(step){if(!hits.length)return;hits[current]?.classList.remove('search-current');current=(current+step+hits.length)%hits.length;const hit=hits[current];hit.classList.add('search-current');hit.scrollIntoView({block:'center',inline:'nearest'});counter.textContent=(current+1)+' / '+hits.length+(limited?'+':'');}
  function search(){
   clearTimeout(searchTimer);stopWorker();const version=++request,codes=header.parentElement.querySelectorAll('.log-text code');marker=new Mark(codes);
   marker.unmark({done:()=>{
    if(version!==request)return;hits=[];current=-1;limited=false;counter.textContent='0 / 0';input.setCustomValidity('');if(!input.value)return;
    const options={className:'log-search-hit',separateWordSearch:false,caseSensitive:sensitive.checked,diacritics:false,acrossElements:true,filter:()=>hits.length<2000,each:hit=>hits.push(hit),done:()=>{if(version===request){limited=limited||hits.length>=2000;select(1)}}};
    if(!regex.checked){marker.mark(input.value,options);return}
    if(input.value.length>512){counter.textContent='正则表达式最多 512 字符';return}
    counter.textContent='查询中…';
    try{
     worker=new Worker('log-search-worker.js');workerTimer=setTimeout(()=>{if(version!==request)return;stopWorker();request++;counter.textContent='查询超时，请简化正则表达式'},2000);
     worker.onmessage=event=>{if(version!==request)return;stopWorker();const result=event.data;if(result.error){counter.textContent='正则表达式无效';input.setCustomValidity('正则表达式无效');return}limited=result.limited;counter.textContent='0 / 0';marker.markRanges(result.ranges,options)};
     worker.onerror=()=>{if(version!==request)return;stopWorker();counter.textContent='查询失败，请重试'};
     worker.postMessage({texts:[...codes].map(code=>code.textContent),pattern:input.value,sensitive:sensitive.checked});
    }catch{stopWorker();counter.textContent='正则查询不可用'}
   }});
  }
  function open(){panel.hidden=false;input.focus();input.select()}
  function close(){panel.hidden=true;request++;clearTimeout(searchTimer);stopWorker();marker?.unmark();hits=[];current=-1;header.parentElement.focus()}
  toolbar.append(logIcon('Search','查找日志内容',open));
  panel.append(input,logIcon('ArrowLeft','上一个匹配',()=>select(-1)),logIcon('ArrowRight','下一个匹配',()=>select(1)),counter,caseLabel,regexLabel,logIcon('X','关闭查找',close));header.append(panel);
  input.oninput=()=>{clearTimeout(searchTimer);stopWorker();request++;searchTimer=setTimeout(search,150)};sensitive.onchange=regex.onchange=search;
  target.onkeydown=event=>{
   if((event.ctrlKey||event.metaKey)&&event.key.toLowerCase()==='f'){event.preventDefault();open()}
   else if(!panel.hidden&&event.key==='Escape'){event.preventDefault();event.stopPropagation();close()}
   else if(!panel.hidden&&(event.key==='Enter'||event.key==='F3')){event.preventDefault();select(event.shiftKey?-1:1)}
  };
  header.onkeydown=event=>{target.onkeydown(event);event.stopPropagation()};
  return {setKeyword(value){panel.hidden=!value;input.value=value;regex.checked=false;sensitive.checked=false;search()}};
 };
})();
