const { app, BrowserWindow, shell } = require('electron');
const path = require('path');
const http = require('http');

let mainWindow;
let gatewayProcess;

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1400,
    height: 900,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true
    },
    title: 'RTCM Gateway UI',
    autoHideMenuBar: true,
    icon: path.join(__dirname, 'icon.ico')
  });

  const isDev = process.argv.includes('--dev');
  
  if (isDev) {
    mainWindow.loadURL('http://localhost:5173');
    mainWindow.webContents.openDevTools();
  } else {
    const distPath = path.join(__dirname, 'dist', 'dist', 'index.html');
    mainWindow.loadFile(distPath);
  }

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

function startGateway() {
  const gatewayPath = path.join(__dirname, '..', 'gateway.exe');
  const { spawn } = require('child_process');
  
  try {
    gatewayProcess = spawn(gatewayPath, [], {
      detached: true,
      stdio: 'ignore',
      cwd: path.dirname(gatewayPath)
    });
    
    gatewayProcess.unref();
    console.log('Gateway started with PID:', gatewayProcess.pid);
  } catch (err) {
    console.log('Gateway not found, will use network mode only');
  }
}

function checkGatewayConnection() {
  return new Promise((resolve) => {
    const req = http.get('http://localhost:8080/api/v1/health', (res) => {
      resolve(true);
    });
    req.on('error', () => resolve(false));
    req.setTimeout(1000, () => {
      req.destroy();
      resolve(false);
    });
  });
}

app.whenReady().then(async () => {
  const gatewayRunning = await checkGatewayConnection();
  
  if (!gatewayRunning) {
    console.log('Starting embedded gateway...');
    startGateway();
    await new Promise(r => setTimeout(r, 3000));
  }
  
  createWindow();
  
  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

app.on('before-quit', () => {
  if (gatewayProcess) {
    gatewayProcess.kill();
  }
});
