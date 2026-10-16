const {chromium}=require('playwright');
const {spawn}=require('child_process');
const path=require('path'),fs=require('fs'),assert=require('assert');

async function main(){
 const root=path.resolve(__dirname,'..'),bin=process.env.LR_TEST_BINARY||path.join(root,'dist','log-reviewer-windows-amd64.exe');
 const child=spawn(bin,['gui','--state-file',path.join(require('os').tmpdir(),'lr-'+require('crypto').randomUUID()+'.json'),'--tls-file',path.join(require('os').tmpdir(),'lr-'+require('crypto').randomUUID()+'.pem'),'--host','127.0.0.1','--https-port','0','--no-browser','--port','0'],{cwd:root,windowsHide:true});
 let output='',browser,context;
 try{
  const url=await new Promise((resolve,reject)=>{const timer=setTimeout(()=>reject(Error('startup timeout')),10000);child.stdout.on('data',b=>{output+=b;const match=output.match(/HTTP: (http:\/\/\S+)/);if(match){clearTimeout(timer);resolve(match[1])}});child.on('error',reject)});
  browser=await chromium.connectOverCDP('http://127.0.0.1:9222',{timeout:5000});context=await browser.newContext({viewport:{width:1440,height:980},timezoneId:'Asia/Shanghai'});
  const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));await page.goto(url,{timeout:10000});
  await page.locator('#upload').setInputFiles([
   {name:'first.log',mimeType:'text/plain',buffer:Buffer.from('2026-10-14T04:30:00.123Z ERROR early\n2026-10-14T04:30:00.999999999Z ERROR included\n')},
   {name:'second.log',mimeType:'text/plain',buffer:Buffer.from('2026-10-14T04:30:01Z ERROR outside\n')}
  ]);await page.waitForFunction(()=>document.getElementById('file-count').textContent==='2 个文件 / 路径');
  await page.locator('#paste-button').click();await page.locator('#paste-text').fill('2026-10-14T04:30:02Z WARN 粘贴日志 : paste warning\n');await page.locator('#confirm-paste').click();await page.waitForFunction(()=>!document.getElementById('paste-dialog').open&&document.getElementById('file-count').textContent==='3 个文件 / 路径');
  await page.locator('#analyze').click();await page.waitForFunction(()=>document.querySelectorAll('.group-row').length===4);
  const previousReport=await page.evaluate(()=>report);let releaseStatus,seenStatus;const release=new Promise(resolve=>releaseStatus=resolve),seen=new Promise(resolve=>seenStatus=resolve);let held=false;
  const holdStatus=async route=>{if(held){await route.continue();return}held=true;const response=await route.fetch({timeout:10000});seenStatus();await release;await route.fulfill({response})};
  await page.route('**/api/status',holdStatus);await page.evaluate(()=>{poll()});await new Promise((resolve,reject)=>{const timer=setTimeout(()=>reject(Error('held status timeout')),10000);seen.then(()=>{clearTimeout(timer);resolve()})});await page.locator('#analyze').click();await page.waitForFunction(()=>busy&&document.querySelectorAll('.group-row').length===0);releaseStatus();await page.waitForFunction(old=>report!==old&&document.querySelectorAll('.group-row').length===4,previousReport);await page.unroute('**/api/status',holdStatus);
  async function choose(id){await page.locator('#'+id).click();await page.getByRole('button',{name:'2026-10-14',exact:true}).click();assert(await page.locator('#time-picker').evaluate(e=>e.open));await page.locator('#time-hour').fill('12');await page.locator('#time-minute').fill('30');await page.locator('#time-second').fill('0');await page.locator('#confirm-time').click();assert(!(await page.locator('#time-picker').evaluate(e=>e.open)))}
  await choose('from');await choose('to');await page.locator('#search-button').click();await page.waitForFunction(()=>document.querySelectorAll('.group-row').length===2);assert((await page.locator('#groups').innerText()).includes('included'));assert(!(await page.locator('#groups').innerText()).includes('outside'));
  const start=await page.locator('#from').inputValue();await page.locator('#from').click();await page.locator('#time-hour').fill('1');await page.locator('#cancel-time').click();assert(await page.locator('#from').inputValue()===start);
  await page.locator('#keyword').fill('included');await page.locator('#regex').fill('early|included');await page.locator('#search-button').click();await page.waitForFunction(()=>document.querySelectorAll('.group-row').length===1);
  await page.getByRole('button',{name:'重置关键字',exact:true}).click();await page.waitForFunction(()=>document.querySelectorAll('.group-row').length===2);assert(await page.locator('#regex').inputValue()==='early|included');assert(await page.locator('#from').inputValue()===start);
  await page.getByRole('button',{name:'重置时间',exact:true}).click();assert(await page.locator('#from').inputValue()==='');assert(await page.locator('#to').inputValue()==='');assert(await page.locator('#regex').inputValue()==='early|included');
  await page.getByRole('button',{name:'重置正则',exact:true}).click();await page.waitForFunction(()=>document.querySelectorAll('.group-row').length===4);
  await page.locator('#source').selectOption({label:'first.log'});await page.locator('#search-button').click();await page.waitForFunction(()=>document.querySelectorAll('.group-row').length===2);await page.getByRole('button',{name:'重置文件范围',exact:true}).click();await page.waitForFunction(()=>document.querySelectorAll('.group-row').length===4);
  await page.locator('#keyword').fill('early');await page.locator('#regex').fill('early');await page.locator('#reset').click();await page.waitForFunction(()=>document.querySelectorAll('.group-row').length===4);assert(await page.locator('#keyword').inputValue()==='');assert(await page.locator('#regex').inputValue()==='');
  fs.mkdirSync(path.join(root,'reports'),{recursive:true});await page.screenshot({path:path.join(root,'reports','filters-desktop.png'),fullPage:true});
  await page.setViewportSize({width:390,height:844});await page.locator('#to').click();await page.screenshot({path:path.join(root,'reports','filters-mobile.png'),fullPage:true});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));assert(await page.evaluate(()=>[...document.querySelectorAll('#time-picker input,#time-picker button,#time-picker select')].every(e=>e.getBoundingClientRect().right<=innerWidth)));assert(errors.length===0,errors.join('\n'));
  console.log('Filter UI passed: nanosecond end bounds, timezone, date confirm/cancel, individual/all reset, multi-upload, pasted UTF-8, desktop/mobile');
 }finally{if(context)await context.close();if(browser)await browser.close();child.kill()}
}
main().catch(e=>{console.error(e);process.exitCode=1});
