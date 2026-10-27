// Recheck failed native cases after external/native window geometry changes.
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');const fs=require('node:fs');const path=require('node:path');
(async()=>{
 const output=path.resolve(__dirname,'../../../docs/desktop');const file=path.join(output,'ux-validation.json');const data=JSON.parse(fs.readFileSync(file));
 const b=await chromium.connectOverCDP('http://127.0.0.1:9333');const p=b.contexts()[0].pages()[0];const c=await p.context().newCDPSession(p);const {windowId}=await c.send('Browser.getWindowForTarget');
 const inspect=()=>p.evaluate(()=>{const el=document.querySelector('.content');return {viewport:[innerWidth,innerHeight],dpr:devicePixelRatio,height:el.clientHeight,scrollHeight:el.scrollHeight,top:el.scrollTop,range:el.scrollHeight-el.clientHeight,pageOverflow:document.documentElement.scrollWidth-innerWidth,cardOverflow:[...document.querySelectorAll('.card,.review-card')].filter(e=>e.scrollWidth>e.clientWidth+2).map(e=>e.className),shellHeight:document.querySelector('.app-shell').clientHeight,focus:document.activeElement?.id}});
 for(let i=0;i<data.rows.length;i++){
  const old=data.rows[i];if(old.pass)continue;
  await c.send('Browser.setWindowBounds',{windowId,bounds:{width:Math.round(old.physicalTarget[0]/data.hostDpiRatio),height:Math.round(old.physicalTarget[1]/data.hostDpiRatio)}});await p.waitForTimeout(500);
  await p.keyboard.press(`Control+Shift+${old.theme==='dark'?'D':'L'}`);await p.keyboard.press(`Control+${['overview','processes','storage','developer','settings'].indexOf(old.screen)+1}`);await p.waitForTimeout(150);
  const before=await inspect(),base=`${old.screen}-${old.theme}-${old.physicalTarget[0]}x${old.physicalTarget[1]}`;
  await p.screenshot({path:path.join(output,'screenshots/ux-polish',base+'.png')});const r=await p.locator('.content').boundingBox();await p.mouse.move(r.x+8,r.y+r.height/2);await p.mouse.wheel(0,10000);await p.waitForTimeout(300);const wheel=await inspect();
  await p.locator('.content').focus();await p.keyboard.press('Control+Home');await p.waitForFunction(()=>document.querySelector('.content').scrollTop<=1);await p.keyboard.press('PageDown');await p.waitForTimeout(300);const keyboard=await inspect();await p.keyboard.press('Control+End');await p.waitForTimeout(300);const bottom=await inspect();await p.screenshot({path:path.join(output,'screenshots/ux-polish',base+'-bottom.png')});
  const pass=before.cardOverflow.length===0&&bottom.cardOverflow.length===0&&before.pageOverflow<=1&&before.shellHeight<=before.viewport[1]+1&&(before.range<=1||wheel.top>0)&&(keyboard.range<=1||keyboard.top>0)&&Math.abs(bottom.top-bottom.range)<=2&&before.viewport.join()===bottom.viewport.join();
  (data.retests||=[]).push({reason:old.recheckReason||'Window geometry changed during the original input sequence; rechecked at the requested target.',original:old});data.rows[i]={...old,before,wheel,keyboard,bottom,pass};
 }
 fs.writeFileSync(file,JSON.stringify(data,null,2));console.log({cases:data.rows.length,passed:data.rows.filter(r=>r.pass).length,retests:data.retests?.length||0});await b.close();
})().catch(error=>{console.error(error);process.exitCode=1});
