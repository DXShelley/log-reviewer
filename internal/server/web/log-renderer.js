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
  {scope:'location',match:/\b[\w$./\\-]+\.(?:java|kt|scala|py|js|ts|go|rs|cs|cpp|c|rb|php):\d+(?::\d+)?/},
  {scope:'exception',match:/\b[\w.$]*(?:Exception|Error|Panic)\b/},
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
  if(highlighter){try{code.innerHTML=highlighter.highlight(text.slice(0,end),{language:'log',ignoreIllegals:true}).value}catch{code.textContent=text.slice(0,end)}}else code.textContent=text.slice(0,end);
  if(end<text.length)code.append(document.createTextNode(text.slice(end)));
  return pre;
 };
})();
