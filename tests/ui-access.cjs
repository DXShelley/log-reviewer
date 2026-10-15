const {chromium}=require('playwright');
const {spawn}=require('child_process');
const fs=require('fs'),os=require('os'),path=require('path'),assert=require('assert');

async function main(){
 const dir=fs.mkdtempSync(path.join(os.tmpdir(),'lr-access-ui-'));
 const binary=process.env.LR_TEST_BINARY||path.resolve(__dirname,'../dist/log-reviewer-windows-amd64.exe');
 const state=path.join(dir,'state.json');
 const rules=path.join(dir,'rules');fs.mkdirSync(rules);
 let child,browser,context;
 async function stop(){if(child){const current=child;child=null;if(current.exitCode!==null||current.signalCode!==null)return;await new Promise(resolve=>{current.once('exit',resolve);current.kill()})}}
 async function start(extra=[]){
  child=spawn(binary,['gui','--host','127.0.0.1','--port','0','--https-port','0','--tls-file',path.join(dir,'tls.pem'),'--no-browser','--state-file',state,'--rule-dir',rules,...extra],{windowsHide:true});
  child.stderr.on('data',b=>process.stderr.write(b));
  return new Promise((resolve,reject)=>{let output='';const timer=setTimeout(()=>reject(Error('startup timeout')),10000);child.on('error',reject);child.on('exit',code=>{clearTimeout(timer);reject(Error('startup exit '+code+' '+output))});child.stdout.on('data',b=>{output+=b;const match=output.match(/HTTP: (http:\/\/\S+)/);if(match){clearTimeout(timer);resolve(match[1])}})});
 }
 try{
  let url=await start();
  browser=await chromium.connectOverCDP('http://127.0.0.1:9222',{timeout:5000});
  context=await browser.newContext({viewport:{width:1440,height:980}});
  const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto(url,{timeout:10000});assert((await page.title()).includes('Log Reviewer'));
  await page.locator('#access-open').click();await page.locator('#access-save').waitFor({state:'visible'});
  await page.waitForFunction(()=>!document.getElementById('access-save').disabled);
  assert(await page.locator('#access-unrestricted').isChecked());
  await page.locator('#access-unrestricted').uncheck();await page.locator('#access-ips').fill('192.0.2.1');await page.locator('#access-save').click();
  await page.locator('#access-error').waitFor({state:'visible'});assert(!fs.existsSync(state));
  await page.locator('#access-ips').fill('127.0.0.1\n10.20.0.0/24');await page.locator('#access-save').click();
  await page.waitForFunction(()=>!document.getElementById('access-dialog').open);
  assert.deepStrictEqual(JSON.parse(fs.readFileSync(state)).allowed_client_ips,['127.0.0.1','10.20.0.0/24']);
  await page.locator('#access-open').click();await page.waitForFunction(()=>!document.getElementById('access-save').disabled);
  await page.screenshot({path:path.join(dir,'desktop.png')});
  await page.setViewportSize({width:390,height:844});await page.screenshot({path:path.join(dir,'mobile.png')});
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  const bounds=await page.locator('#access-dialog').boundingBox();assert(bounds.x>=0&&bounds.x+bounds.width<=390);
  const oldToken=new URL(url).hash.slice(1);await stop();url=await start();
  await page.goto(url,{timeout:10000});await page.locator('#access-open').click();await page.waitForFunction(()=>!document.getElementById('access-save').disabled);
  assert.strictEqual(await page.locator('#access-ips').inputValue(),'127.0.0.1\n10.20.0.0/24');
  const oldStatus=await page.evaluate(async old=>{const r=await fetch('/api/status',{headers:{Authorization:'Bearer '+old}});return r.status},oldToken);assert.strictEqual(oldStatus,401);
  await page.evaluate(()=>{sessionStorage.clear();history.replaceState(null,'',location.pathname)});await page.reload();
  await page.locator('#access-dialog[open]').waitFor();await page.locator('#access-token').fill(new URL(url).hash.slice(1));await page.locator('#access-auth').click();await page.waitForFunction(()=>!document.getElementById('access-save').disabled);
  assert.strictEqual(await page.locator('#access-token').inputValue(),'');
  await page.locator('#access-unrestricted').check();await page.locator('#access-save').click();await page.waitForFunction(()=>!document.getElementById('access-dialog').open);
  assert.deepStrictEqual(JSON.parse(fs.readFileSync(state)).allowed_client_ips,[]);
  await stop();url=await start(['--allowed-client-ips','192.0.2.1']);
  assert.strictEqual((await fetch(url,{signal:AbortSignal.timeout(5000)})).status,403);
  await stop();url=await start();assert.strictEqual((await fetch(url,{signal:AbortSignal.timeout(5000)})).status,403);
  await stop();url=await start(['--allowed-client-ips','off']);assert.strictEqual((await fetch(url,{signal:AbortSignal.timeout(5000)})).status,200);
  assert.deepStrictEqual(errors,[]);
  console.log('PASS: GUI save, lockout prevention, restart persistence, token entry, old token rejected, disable, desktop/mobile. Evidence: '+dir);
 }finally{if(context)await context.close();if(browser)await browser.close();await stop()}
}
main().catch(e=>{console.error(e);process.exitCode=1});
