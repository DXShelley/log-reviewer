const {chromium}=require('playwright');
const {spawn}=require('child_process');
const fs=require('fs'),os=require('os'),path=require('path'),assert=require('assert');

function startup(child,parse){return new Promise((resolve,reject)=>{
 let output='';const timer=setTimeout(()=>reject(Error('startup timeout')),10000);
 child.once('error',reject);child.stdout.on('data',data=>{output+=data;const result=parse(output);if(result){clearTimeout(timer);resolve(result)}});
})}
async function stop(child){if(!child||child.exitCode!==null||child.signalCode!==null)return;await new Promise(resolve=>{child.once('exit',resolve);child.kill()})}

async function main(){
 const root=path.resolve(__dirname,'..'),temporary=fs.mkdtempSync(path.join(os.tmpdir(),'lr-import-conflicts-'));
 const contents=['2026-10-15 12:30:00 ERROR app : first\n','2026-10-15 12:30:01 WARN app : second\n','2026-10-15 12:30:02 ERROR app : third\n'];
 const local=['a/app.log','copy/duplicate.log','b/app.log','c/app.log'].map((name,i)=>{const file=path.join(temporary,name);fs.mkdirSync(path.dirname(file),{recursive:true});fs.writeFileSync(file,contents[i===1?0:i===0?0:i-1]);return file});
 const original=local.map(file=>fs.readFileSync(file));
 const binary=process.env.LR_TEST_BINARY||path.join(root,'dist/log-reviewer-windows-amd64.exe');
 const fixture=spawn(process.env.LR_SFTP_FIXTURE_BINARY||path.join(root,'.work/sftp-fixture.exe'),[],{windowsHide:true});
 const gui=spawn(binary,['gui','--host','127.0.0.1','--no-browser','--port','0','--https-port','0','--tls-file',path.join(temporary,'tls.pem'),'--state-file',path.join(temporary,'state.json'),'--rule-dir',path.join(temporary,'rules')],{cwd:temporary,windowsHide:true});
 let browser,context;
 try{
  const [remote,url]=await Promise.all([startup(fixture,text=>{try{return JSON.parse(text)}catch{return null}}),startup(gui,text=>text.match(/HTTP: (http:\/\/\S+)/)?.[1])]);
  fs.mkdirSync(path.join(remote.root,'duplicates'));fs.writeFileSync(path.join(remote.root,'duplicates','remote-a.log'),contents[0]);fs.writeFileSync(path.join(remote.root,'duplicates','remote-b.log'),contents[0]);
  browser=await chromium.connectOverCDP('http://127.0.0.1:9222',{timeout:5000});context=await browser.newContext({viewport:{width:1440,height:1000}});
  const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));page.on('console',m=>{if(m.type()==='error')errors.push(m.text())});
  await page.goto(url,{timeout:10000});assert((await page.title()).includes('Log Reviewer'));await page.locator('#upload-button').waitFor();
  let selected=[];await page.route('**/api/pick-paths?kind=files',route=>route.fulfill({json:{paths:selected}}));
  async function pick(paths){selected=paths;await page.locator('#upload-button').click();await Promise.all([page.waitForResponse(response=>response.url().includes('/api/pick-paths?kind=files')),page.locator('#import-files').click()])}
  async function settled(){await page.waitForFunction(()=>!importing)}
  await pick([local[0]]);await settled();assert.equal(await page.locator('.file-chip').count(),1);
  await pick([local[1]]);await settled();assert.equal(await page.locator('.file-chip').count(),1);
  await pick([local[2]]);const conflict=page.getByRole('dialog').filter({has:page.getByRole('heading',{name:'同名文件已存在'})});await conflict.waitFor();
  assert.equal(await conflict.getByLabel('保留文件名称').inputValue(),'app (1).log');await page.screenshot({path:path.join(temporary,'conflict-desktop.png')});
  await conflict.getByRole('button',{name:'保留并重命名',exact:true}).click();await settled();assert.equal(await page.locator('.file-chip').count(),2);assert((await page.locator('#files').innerText()).includes('app (1).log'));
  await pick([local[3]]);await conflict.waitFor();await conflict.getByRole('button',{name:'取消',exact:true}).click();await settled();assert.equal(await page.locator('.file-chip').count(),2);
  await pick([local[3]]);await conflict.waitFor();await conflict.getByRole('button',{name:'覆盖',exact:true}).click();await settled();assert.deepEqual(await page.evaluate(()=>files),[local[2],local[3]]);
  await page.evaluate(()=>{files=[];uploads=[];pathImports.clear();renderFiles()});
  await page.locator('#path-toggle').click();await page.locator('#paths').fill(local.slice(0,3).join('\n'));await page.locator('#add-paths').click();await conflict.waitFor();
  await conflict.getByLabel('保留文件名称').fill('manual.log');await conflict.getByRole('button',{name:'保留并重命名',exact:true}).click();await settled();assert.equal(await page.locator('.file-chip').count(),2);assert((await page.locator('#files').innerText()).includes('manual.log'));
  await page.evaluate(async text=>{await uploadList([new File([text],'uploaded-copy.log')])},contents[0]);assert.equal(await page.locator('.file-chip').count(),2);assert.equal(await page.evaluate(()=>uploads.length),0);
  await page.locator('#remote-button').click();
  for(const [field,value]of Object.entries({host:remote.host,port:String(remote.port),user:remote.user,password:remote.password,path:'duplicates',fingerprint:remote.fingerprint}))await page.locator('#remote-'+field).fill(value);
  await page.locator('#remote-submit').click();await page.locator('#remote-download').waitFor();await page.locator('#remote-download').click();await page.waitForFunction(()=>document.getElementById('remote-status').textContent.includes('已导入 0 个文件，跳过 2 个文件'));
  await page.locator('#close-remote').click();assert.equal(await page.locator('.file-chip').count(),2);
  await page.locator('#analyze').click();await page.waitForFunction(()=>!busy&&groups.length===2);const report=await page.evaluate(()=>json('report'));assert.equal(report.events,2);assert.equal(report.sources.length,2);
  await page.evaluate(()=>{files=[];uploads=[];pathImports.clear();renderFiles()});
  await page.locator('#remote-button').click();await page.locator('#remote-submit').click();await page.locator('#remote-download').waitFor();await page.locator('#remote-download').click();await page.waitForFunction(()=>document.getElementById('remote-status').textContent.includes('已导入 1 个文件，跳过 1 个文件'));await page.locator('#close-remote').click();
  await pick([local[0]]);await settled();assert.equal(await page.locator('.file-chip').count(),1);assert.equal(await page.evaluate(()=>files.length),0);
  for(let i=0;i<local.length;i++)assert(fs.readFileSync(local[i]).equals(original[i]));
  assert(fs.readFileSync(path.join(remote.root,'duplicates','remote-a.log'),'utf8')===contents[0]);
  await page.setViewportSize({width:390,height:844});await pick([local[2]]);await settled();assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await page.screenshot({path:path.join(temporary,'imports-mobile.png'),fullPage:true});
  assert.deepEqual(errors,[]);console.log('PASS: native/path same-content dedup, default/custom alias, overwrite/cancel, upload/local/remote cross-entry dedup, remote duplicate content, accurate event counts, unchanged sources, desktop/mobile. Evidence: '+temporary);
 }finally{if(context)await context.close();if(browser)await browser.close();await stop(gui);await stop(fixture)}
}
main().catch(e=>{console.error(e);process.exitCode=1});
