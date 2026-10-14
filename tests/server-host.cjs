const {spawn}=require('child_process');
const {chromium}=require('playwright');
const net=require('net'),http=require('http'),os=require('os'),path=require('path'),fs=require('fs'),assert=require('assert');
async function request(url,token,origin){return new Promise((resolve,reject)=>{const req=http.get(url,{headers:{...(token?{Authorization:'Bearer '+token}:{}),...(origin?{Origin:origin}:{})},timeout:5000},res=>{res.resume();res.on('end',()=>resolve(res.statusCode))});req.on('error',reject);req.on('timeout',()=>req.destroy(Error('timeout')))})}
async function main(){
 const root=path.resolve(__dirname,'..'),hold=net.createServer();
 await new Promise(resolve=>hold.listen(0,'0.0.0.0',resolve));const occupied=hold.address().port;
 const child=spawn(path.join(root,'dist/log-reviewer-windows-amd64.exe'),['gui','--no-browser','--port',String(occupied),'--rule-dir',fs.mkdtempSync(path.join(os.tmpdir(),'lr-host-rules-'))],{windowsHide:true});
 let output='',stderr='',browser,context;
 child.stderr.on('data',b=>stderr+=b);
 try {
  const url=await new Promise((resolve,reject)=>{const t=setTimeout(()=>reject(Error('startup timeout')),10000);child.on('error',reject);child.stdout.on('data',b=>{output+=b;const m=output.match(/GUI: (http:\/\/\S+)/);if(m){clearTimeout(t);resolve(m[1])}})});
  const parsed=new URL(url),token=parsed.hash.slice(1);assert(+parsed.port!==occupied);assert(stderr.includes('automatically assigned'));
  assert(await request(parsed.origin+'/api/status','')===401);
  assert(await request(parsed.origin+'/api/status',token,'http://evil.example')===403);
  assert(await request(parsed.origin+'/api/status',token,parsed.origin)===200);
  await new Promise(resolve=>setTimeout(resolve,100));
  if(output.includes('Internal IPv6:'))assert(await request('http://[::1]:'+parsed.port+'/api/status',token,'http://[::1]:'+parsed.port)===200);
  const ipv4=Object.values(os.networkInterfaces()).flat().find(a=>a.family==='IPv4'&&!a.internal);
  const origin=ipv4?'http://'+ipv4.address+':'+parsed.port:parsed.origin;
  browser=await chromium.connectOverCDP('http://127.0.0.1:9222',{timeout:5000});context=await browser.newContext();const page=await context.newPage();await page.goto(origin+'/#'+token,{timeout:10000});await page.locator('#upload').setInputFiles({name:'remote.log',mimeType:'text/plain',buffer:Buffer.from('2026-10-14 12:30:00 ERROR remote failed\n')});await page.waitForFunction(()=>!document.getElementById('analyze').disabled);await page.locator('#analyze').click();await page.waitForFunction(()=>document.querySelectorAll('.group-row').length===1);
  assert(output.includes('External IPv4')||!ipv4);console.log('Server host passed: occupied port fallback, wildcard IPv4, authentication, cross-origin rejection, IPv6 when available, remote browser upload and analysis');
 }finally {if(context)await context.close();if(browser)await browser.close();child.kill();hold.close()}
}
main().catch(e=>{console.error(e);process.exitCode=1});
