const {chromium}=require('playwright');
const {spawn}=require('child_process');
const fs=require('fs'),os=require('os'),path=require('path'),assert=require('assert');

function startup(child,parse){return new Promise((resolve,reject)=>{
 let text='';const timer=setTimeout(()=>reject(Error('startup timeout')),10000);
 child.on('error',reject);child.stdout.on('data',b=>{text+=b;const value=parse(text);if(value){clearTimeout(timer);resolve(value)}});
})}

async function main(){
 const root=path.resolve(__dirname,'..'),temporary=fs.mkdtempSync(path.join(os.tmpdir(),'lr-settings-ui-'));
 const binary=process.env.LR_TEST_BINARY||path.join(root,'dist/log-reviewer-windows-amd64.exe');
 const fixture=spawn(process.env.LR_SFTP_FIXTURE_BINARY||path.join(root,'.work/sftp-fixture.exe'),[],{windowsHide:true});
 const stateFile=path.join(temporary,'state.json');
 const gui=spawn(binary,['gui','--host','127.0.0.1','--no-browser','--port','0','--https-port','0','--tls-file',path.join(temporary,'tls.pem'),'--state-file',stateFile,'--rule-dir',path.join(temporary,'rules')],{windowsHide:true});
 let browser,context;
 try{
  const [remote,url]=await Promise.all([startup(fixture,text=>{try{return JSON.parse(text)}catch{return null}}),startup(gui,text=>text.match(/(?:HTTP|GUI): (http:\/\/\S+)/)?.[1])]);
  fs.writeFileSync(path.join(remote.root,'logs','service.text'),'2026-10-15 12:30:00 ERROR service : text format test\n');
  browser=await chromium.connectOverCDP('http://127.0.0.1:9222',{timeout:5000});context=await browser.newContext({viewport:{width:1440,height:1000}});
  const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));page.on('console',m=>{if(m.type()==='error')errors.push(m.text())});
  await page.goto(url,{timeout:10000});assert((await page.title()).includes('Log Reviewer'));await page.locator('#path-toggle').waitFor();
  async function settings(directory,patterns){
   await page.locator('#access-open').click();await page.locator('#settings-tab-1').click();
   await page.locator('#remote-save-directory').fill(directory);if(patterns)await page.locator('#file-patterns').fill(patterns);
   await page.locator('#file-settings-save').click();await page.locator('#access-dialog').waitFor({state:'hidden'});
  }
  const first=path.join(temporary,'first'),second=path.join(temporary,'second');
  await settings(first);assert(fs.readFileSync(stateFile,'utf8').includes('*.text'));
  const downloaded=await page.evaluate(async options=>{
   await json('remote-fetch',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(options)});
   const limit=Date.now()+15000;let state;
   do{await new Promise(resolve=>setTimeout(resolve,30));state=await json('remote-fetch');if(Date.now()>limit)throw Error('remote timeout')}while(state.busy);
   if(state.error)throw Error(state.error);for(const file of state.downloads)await acceptImport(file);
   return {state,settings:await json('import-settings')};
  },{...remote,root:undefined,private_key:undefined,recursive:true,ssh_dir:path.join(temporary,'ssh')});
  assert.equal(downloaded.state.downloads.length,3);
  const oldSession=downloaded.settings.session_directory;
  const task=path.join(oldSession,downloaded.state.id),manifest=JSON.parse(fs.readFileSync(path.join(task,'manifest.json'),'utf8'));
  assert.deepEqual(manifest.files.map(f=>path.basename(f.local_path)).sort(),['app.log','service.text','worker.log']);
  const before=await page.locator('#files').innerText(),titles=await page.locator('#files span').evaluateAll(items=>items.map(e=>e.title));
  await page.locator('#analyze').click();await page.waitForFunction(()=>!busy&&groups.length===3);
  await settings(second,'*.text');
  assert(!fs.existsSync(oldSession));assert.equal(await page.locator('#files').innerText(),before);assert.deepEqual(await page.locator('#files span').evaluateAll(items=>items.map(e=>e.title)),titles);
  const after=await page.evaluate(()=>json('import-settings'));
  assert(fs.existsSync(path.join(after.session_directory,downloaded.state.id,'app.log')));
  const detail=await page.evaluate(()=>json('details?group='+groups.find(g=>g.level==='ERROR').id));assert(detail.items?.length||detail.details?.length||JSON.stringify(detail).includes('remote failed')||JSON.stringify(detail).includes('text format test'));
  assert.deepEqual(JSON.parse(fs.readFileSync(stateFile,'utf8')).file_patterns,['*.text']);
  const displayNames=await page.locator('#files span').evaluateAll(items=>items.map(e=>e.textContent+'|'+e.title).sort());
  await page.reload({timeout:10000});await page.waitForFunction(()=>groups.length===3);
  assert.deepEqual(await page.locator('#files span').evaluateAll(items=>items.map(e=>e.textContent+'|'+e.title).sort()),displayNames);
  const reloadNames=await page.locator('#files').innerText(),reloadTitles=await page.locator('#files span').evaluateAll(items=>items.map(e=>e.title));
  await settings(path.join(temporary,'third'),'*.text');
  assert.equal(await page.locator('#files').innerText(),reloadNames);assert.deepEqual(await page.locator('#files span').evaluateAll(items=>items.map(e=>e.title)),reloadTitles);
  await page.locator('#analyze').click();await page.waitForFunction(()=>!busy&&groups.length===1);
  await page.locator('#remote-button').click();assert.equal(await page.locator('#remote-pattern').inputValue(),'*.text');
  for(const field of ['host','port','user','path','fingerprint','password'])await page.locator('#remote-'+field).fill(String(remote[field]));
  await page.locator('#remote-ssh-dir').fill(path.join(temporary,'ssh'));await page.locator('#remote-submit').click();
  await page.waitForFunction(()=>document.querySelectorAll('#remote-preview tbody tr').length===1);
  assert((await page.locator('#remote-preview tbody').innerText()).includes('service.text'));await page.locator('#close-remote').click();
  await page.evaluate(()=>{for(let i=0;i<45;i++)uploads.push({id:'layout-'+i,name:'service-node-'+i+'.log',remote_path:'/var/log/node-'+i+'.log'});renderFiles()});
  for(const width of [1440,390]){
   await page.setViewportSize({width,height:1000});
   const layout=await page.locator('#files').evaluate(el=>({rows:new Set([...el.children].map(e=>e.offsetTop)).size,overflow:document.documentElement.scrollWidth>innerWidth,clipped:[...el.children].some(e=>e.getBoundingClientRect().right>innerWidth)}));
   assert(layout.rows>1&&!layout.overflow&&!layout.clipped,JSON.stringify(layout));
   await page.screenshot({path:path.join(temporary,'files-'+width+'.png'),fullPage:true});
  }
  await page.locator('#access-open').click();await page.locator('#settings-tab-1').click();await page.screenshot({path:path.join(temporary,'settings-mobile.png')});
  assert.deepEqual(errors,[]);console.log('File settings passed: persistence, original filenames, .text discovery, migration with usable analysis, unchanged remote names/paths, desktop/mobile wrapping. Evidence: '+temporary);
 }finally{if(context)await context.close();if(browser)await browser.close();gui.kill();fixture.kill()}
}
main().catch(e=>{console.error(e);process.exitCode=1});
