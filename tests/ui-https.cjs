const {chromium}=require('playwright');
const {spawn,execFile}=require('child_process');
const {promisify}=require('util');
const https=require('https'),http=require('http'),net=require('net'),fs=require('fs'),os=require('os'),path=require('path'),assert=require('assert');

function request(url,headers={},trust=true,tlsOptions={}){
 return new Promise((resolve,reject)=>{const req=(url.startsWith('https:')?https:http).get(url,{headers,rejectUnauthorized:trust,...tlsOptions,timeout:5000},res=>{res.resume();res.on('end',()=>resolve(res.statusCode))});req.on('error',reject);req.on('timeout',()=>req.destroy(Error('request timeout')))})
}
async function unusedPort(){const listener=net.createServer();await new Promise((resolve,reject)=>{listener.once('error',reject);listener.listen(0,'127.0.0.1',resolve)});const port=listener.address().port;await new Promise(resolve=>listener.close(resolve));return port}
function rejectsConnection(port){return new Promise((resolve,reject)=>{const socket=net.connect({host:'127.0.0.1',port});socket.setTimeout(5000);socket.on('connect',()=>{socket.destroy();reject(Error('HTTP is still listening'))});socket.on('error',err=>err.code==='ECONNREFUSED'?resolve():reject(err));socket.on('timeout',()=>{socket.destroy();reject(Error('connection timeout'))})})}
async function main(){
 const dir=fs.mkdtempSync(path.join(os.tmpdir(),'lr-https-ui-')),bundle=path.join(dir,'tls.pem'),state=path.join(dir,'state.json');
 const binary=process.env.LR_TEST_BINARY||path.resolve(__dirname,'../dist/log-reviewer-windows-amd64.exe');
 const rules=path.join(dir,'rules');fs.mkdirSync(rules);
 let child,browser,context;
 async function stop(){if(!child)return;const process=child;child=null;if(process.exitCode!==null||process.signalCode!==null)return;await new Promise(resolve=>{process.once('exit',resolve);process.kill()})}
 async function start(extra=[]){
  child=spawn(binary,['gui','--host','127.0.0.1','--port','0','--https-port','0','--no-browser','--state-file',state,'--tls-file',bundle,'--rule-dir',rules,...extra],{windowsHide:true});
  return new Promise((resolve,reject)=>{let output='',errors='';const timer=setTimeout(()=>reject(Error('startup timeout '+errors)),10000);child.stderr.on('data',b=>errors+=b);child.on('error',reject);child.on('exit',code=>{clearTimeout(timer);reject(Error('startup exit '+code+' '+errors))});child.stdout.on('data',b=>{output+=b;const secure=output.match(/GUI: (https:\/\/\S+)/),plain=output.match(/HTTP: (http:\/\/\S+)/);if(secure&&(extra.includes('--http=false')||plain)){clearTimeout(timer);resolve({url:secure[1],plain:plain?.[1],output})}})})
 }
 try{
  let run=await start(['--allowed-client-ips','127.0.0.1']);let url=new URL(run.url),token=url.hash.slice(1),auth={Authorization:'Bearer '+token};
  assert(fs.existsSync(bundle));const original=fs.readFileSync(bundle);
  await assert.rejects(()=>request(url.origin+'/api/status',auth),e=>['DEPTH_ZERO_SELF_SIGNED_CERT','SELF_SIGNED_CERT_IN_CHAIN'].includes(e.code));
  const ca=fs.readFileSync(bundle);
  assert.strictEqual(await request(url.origin+'/api/status',auth,true,{ca}),200);
  await assert.rejects(()=>request(url.origin+'/api/status',auth,true,{ca,servername:'wrong.example.test'}),{code:'ERR_TLS_CERT_ALTNAME_INVALID'});
  for(const version of ['TLSv1.2','TLSv1.3'])assert.strictEqual(await request(url.origin+'/api/status',auth,true,{ca,minVersion:version,maxVersion:version}),200);
  await assert.rejects(()=>request(url.origin+'/api/status',auth,true,{ca,minVersion:'TLSv1.1',maxVersion:'TLSv1.1'}));
  for(const origin of [url.origin,new URL(run.plain).origin]){
   assert.strictEqual(await request(origin+'/api/status',{},false),401);
   assert.strictEqual(await request(origin+'/api/status',{...auth,Origin:origin},false),200);
   assert.strictEqual(await request(origin+'/api/status',{...auth,Origin:'https://evil.example'},false),403);
  }
  browser=await chromium.connectOverCDP('http://127.0.0.1:9222',{timeout:5000});
  // Self-signed certificates are accepted only inside this isolated test context.
  context=await browser.newContext({ignoreHTTPSErrors:true,viewport:{width:1440,height:980}});
  const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));page.on('console',m=>{if(['error','warning'].includes(m.type()))errors.push(m.text())});
  await page.goto(run.url,{timeout:10000});assert((await page.title()).includes('Log Reviewer'));assert(await page.evaluate(()=>isSecureContext));
  await page.locator('#access-open').click();await page.waitForFunction(()=>!document.getElementById('access-save').disabled);assert.strictEqual(await page.locator('#access-ips').inputValue(),'127.0.0.1');
  await page.locator('#access-ips').fill('127.0.0.1\n10.0.0.0/8');await page.locator('#access-save').click();await page.waitForFunction(()=>!document.getElementById('access-dialog').open);
  await page.locator('#upload').setInputFiles({name:'https.log',mimeType:'text/plain',buffer:Buffer.from('2026-10-15 12:30:00 ERROR HTTPS test failed\n')});await page.waitForFunction(()=>!document.getElementById('analyze').disabled);await page.locator('#analyze').click();await page.waitForFunction(()=>document.querySelectorAll('.group-row').length===1);
  await page.screenshot({path:path.join(dir,'https-desktop.png')});await page.setViewportSize({width:390,height:844});await page.screenshot({path:path.join(dir,'https-mobile.png')});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  await stop();const disabledPort=await unusedPort();run=await start(['--http=false','--port',String(disabledPort)]);assert(!run.plain);await rejectsConnection(disabledPort);assert(fs.readFileSync(bundle).equals(original));
  url=new URL(run.url);assert.strictEqual(await request(url.origin+'/api/status',{Authorization:'Bearer '+token},false),401);
  token=url.hash.slice(1);assert.strictEqual(await request(url.origin+'/api/status',{Authorization:'Bearer '+token},false),200);
  await stop();run=await start(['--http=false','--tls-cert',bundle,'--tls-key',bundle]);
  assert.strictEqual(await request(new URL(run.url).origin+'/',{},false),200);assert(fs.readFileSync(bundle).equals(original));
  await stop();run=await start(['--allowed-client-ips','192.0.2.1']);
  assert.strictEqual(await request(new URL(run.url).origin+'/',{},false),403);assert.strictEqual(await request(new URL(run.plain).origin+'/'),403);
  await stop();
  const invalid=path.join(dir,'invalid.pem');fs.writeFileSync(invalid,'invalid certificate');
  for(const extra of [['--tls-cert',bundle],['--tls-cert',invalid,'--tls-key',invalid],['--tls-file',invalid]]){
   await assert.rejects(()=>promisify(execFile)(binary,['gui','--no-browser','--host','127.0.0.1','--https-port','0','--port','0','--state-file',state,'--rule-dir',rules,...extra],{timeout:10000,windowsHide:true}),err=>err.code===1&&!/GUI:|HTTP:/.test(err.stdout));
  }
  assert.deepStrictEqual(errors,[]);
  console.log('PASS: HTTPS/HTTP, trusted certificate and hostname validation, TLS 1.2/1.3, TLS 1.1 rejection, token/origin/IP controls, HTTPS browser workflow, certificate reuse, HTTP port closed, invalid certificates fail closed, desktop/mobile. Evidence: '+dir);
 }finally{if(context)await context.close();if(browser)await browser.close();await stop()}
}
main().catch(e=>{console.error(e);process.exitCode=1});
