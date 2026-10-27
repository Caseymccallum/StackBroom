const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');const fs=require('node:fs');const path=require('node:path');
(async()=>{
 const b=await chromium.connectOverCDP('http://127.0.0.1:9333');const p=b.contexts()[0].pages()[0];const cdp=await p.context().newCDPSession(p);const results={};
 const metrics=()=>p.evaluate(()=>{const e=document.querySelector('.content');return {viewport:[innerWidth,innerHeight],range:e.scrollHeight-e.clientHeight,top:e.scrollTop,overflow:document.documentElement.scrollWidth-innerWidth,focus:document.activeElement?.id}});
 // Emulated 200% scaling on a 1280x720 target; this is not an OS DPI setting change.
 await cdp.send('Emulation.setDeviceMetricsOverride',{width:640,height:360,deviceScaleFactor:2,mobile:false});
 results.scale200=[];
 for(let i=1;i<=5;i++){await p.keyboard.press(`Control+${i}`);await p.waitForTimeout(100);await p.keyboard.press('Control+End');await p.waitForTimeout(300);results.scale200.push({screen:i,...await metrics()});}
 await cdp.send('Emulation.clearDeviceMetricsOverride');
 // Keyboard traversal: headings/controls and focused scrollers stay in the visible viewport.
 results.keyboard=[];
 for(let i=1;i<=5;i++){
  await p.keyboard.press(`Control+${i}`);await p.waitForTimeout(100);
  const steps=[];
  for(let j=0;j<20;j++){await p.keyboard.press('Tab');steps.push(await p.evaluate(()=>{const e=document.activeElement,r=e.getBoundingClientRect();return {tag:e.tagName,name:e.getAttribute('aria-label')||e.textContent?.trim().slice(0,60),visible:r.bottom>0&&r.top<innerHeight&&r.right>0&&r.left<innerWidth}}));}
  results.keyboard.push({screen:i,steps});
 }
 await p.keyboard.press('Control+2');await p.waitForTimeout(100);
 await p.getByRole('textbox',{name:'Search processes'}).focus();await p.keyboard.type('unlikely-stackbroom-process-name');await p.getByText('No processes match',{exact:true}).waitFor();results.searchEmpty=true;
 await p.keyboard.press('Control+A');await p.keyboard.press('Backspace');await p.waitForTimeout(150);
 const table=p.locator('.table-wrap').first();await table.focus();await p.keyboard.press('Control+End');await p.waitForTimeout(200);results.tableKeyboard=await table.evaluate(e=>({top:e.scrollTop,range:e.scrollHeight-e.clientHeight}));
 await table.evaluate(e=>e.scrollTop=0);const box=await table.boundingBox();await p.mouse.move(box.x+50,box.y+60);await p.mouse.wheel(0,10000);await p.waitForTimeout(250);results.tableWheel=await table.evaluate(e=>({top:e.scrollTop,range:e.scrollHeight-e.clientHeight}));
 const next=p.getByRole('button',{name:'Next page',exact:true});await next.focus();await p.keyboard.press('Space');results.pagination=await p.getByText('2 /',{exact:false}).count();
 await p.keyboard.press('Control+4');await p.waitForTimeout(100);const link=p.getByRole('link',{name:'TCP listeners',exact:true});await link.focus();await p.keyboard.press('Enter');results.sectionLink=await p.locator('h1').textContent();
 const summary=p.locator('.observation-notes summary');if(await summary.count()){await summary.focus();await p.keyboard.press('Enter');results.warningExpanded=await p.locator('.observation-notes').evaluate(e=>e.open);}
 await p.emulateMedia({reducedMotion:'reduce'});results.reducedMotion=await p.evaluate(()=>({matches:matchMedia('(prefers-reduced-motion: reduce)').matches,transition:getComputedStyle(document.querySelector('.btn')).transitionDuration}));await p.emulateMedia({reducedMotion:'no-preference'});
 const w=await cdp.send('Browser.getWindowForTarget');await cdp.send('Browser.setWindowBounds',{windowId:w.windowId,bounds:{windowState:'minimized'}});await p.waitForTimeout(250);await cdp.send('Browser.setWindowBounds',{windowId:w.windowId,bounds:{windowState:'normal'}});await p.waitForTimeout(250);results.restored=await metrics();
 fs.writeFileSync(path.resolve(__dirname,'../../../docs/desktop/ux-interactions.json'),JSON.stringify(results,null,2));console.log(JSON.stringify(results));await b.close();
})().catch(e=>{console.error(e);process.exitCode=1});
