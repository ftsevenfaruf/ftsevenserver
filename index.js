const express = require('express');
const cors = require('cors');
const helmet = require('helmet');
const rateLimit = require('express-rate-limit');
const { spawn } = require('child_process');
const path = require('path');

const app = express();
const PORT = 3001;

// Keep track of the currently running Go process
let goProcess = null;

// --- Security headers ---
app.use(helmet());

// --- CORS: only allow localhost:3000 ---
app.use(cors({
  origin: 'http://localhost:3000',
  methods: ['GET', 'POST', 'PUT', 'DELETE'],
  credentials: true,
}));

// --- Rate limiting ---
const limiter = rateLimit({
  windowMs: 15 * 60 * 1000,
  max: 100,
  standardHeaders: true,
  legacyHeaders: false,
});

app.use(limiter);

// --- Body parsing ---
app.use(express.json());


const API_KEY = 'Tarjetazo@40';

function authenticateApiKey(req, res, next) {
  const apiKey = req.header('x-api-key');

  if (apiKey !== API_KEY) {
    return res.status(401).json({
      success: false,
      error: 'Unauthorized',
    });
  }

  next();
}

// --- Go application directory ---
const GO_APP_DIR = path.join(__dirname, 'go-app');

// --- Health check ---
app.get('/', (req, res) => {
  res.send('Hello, world!');
});

// ============================================================
// START GO PROCESS
// ============================================================

app.post('/run' , authenticateApiKey, (req, res) => {
  const { url, amount } = req.body;

  // --- Basic validation ---
  if (!url || typeof url !== 'string') {
    return res.status(400).json({
      success: false,
      error: 'url is required and must be a string',
    });
  }

  if (!amount || typeof amount !== 'string') {
    return res.status(400).json({
      success: false,
      error: 'amount is required and must be a string',
    });
  }

  // --- Validate URL ---
  let parsedUrl;

  try {
    parsedUrl = new URL(url);
  } catch {
    return res.status(400).json({
      success: false,
      error: 'url must be a valid URL',
    });
  }

  if (
    parsedUrl.protocol !== 'http:' &&
    parsedUrl.protocol !== 'https:'
  ) {
    return res.status(400).json({
      success: false,
      error: 'url must use http or https',
    });
  }

  // --- Validate amount ---
  if (!/^\d+$/.test(amount)) {
    return res.status(400).json({
      success: false,
      error: 'amount must be a numeric string',
    });
  }

  // Don't start another Go process if one is already running
  if (goProcess && !goProcess.killed) {
    return res.status(409).json({
      success: false,
      error: 'A Go process is already running',
      pid: goProcess.pid,
    });
  }

  const args = [
    'run',
    'main.go',
    '-u',
    url,
    '-b',
    amount,
  ];

  console.log('');
  console.log('========================================');
  console.log('Starting Go application');
  console.log('========================================');
  console.log(`URL: ${url}`);
  console.log(`Amount: ${amount}`);
  console.log(`Directory: ${GO_APP_DIR}`);
  console.log(`Command: go ${args.join(' ')}`);
  console.log('========================================');

  // --- Start Go process ---
  const child = spawn('go', args, {
    cwd: GO_APP_DIR,
  });

  // Store the process globally so /run/stop can access it
  goProcess = child;

  let stdout = '';
  let stderr = '';
  let responseSent = false;

  // --- Capture stdout ---
  child.stdout.on('data', (data) => {
    const text = data.toString();

    stdout += text;

    console.log(`[go stdout] ${text.trimEnd()}`);
  });

  // --- Capture stderr ---
  child.stderr.on('data', (data) => {
    const text = data.toString();

    stderr += text;

    console.error(`[go stderr] ${text.trimEnd()}`);
  });

  // --- Failed to start ---
  child.on('error', (err) => {
    console.error('Failed to start Go process:', err);

    if (!responseSent) {
      responseSent = true;

      return res.status(500).json({
        success: false,
        error: err.message,
        logs: stdout,
        stderr,
      });
    }
  });

  // --- Return initial response after 5 seconds ---
  setTimeout(() => {
    if (responseSent) {
      return;
    }

    responseSent = true;

    console.log('5 seconds elapsed - returning HTTP response');

    return res.status(200).json({
      success: true,
      message: 'Go application started',
      pid: child.pid,
      logs: stdout,
      stderr,
    });
  }, 5000);

  // --- Go process finished ---
  child.on('close', (code, signal) => {
    console.log(
      `Go process exited. code=${code}, signal=${signal}`
    );

    // Clear the global process reference
    if (goProcess === child) {
      goProcess = null;
    }
  });
});

// ============================================================
// STOP GO PROCESS
// ============================================================

app.post('/run/stop',authenticateApiKey, (req, res) => {
  if (!goProcess) {
    return res.status(404).json({
      success: false,
      error: 'No Go process is currently running',
    });
  }

  const pid = goProcess.pid;

  console.log('');
  console.log('========================================');
  console.log('Stopping Go application');
  console.log(`PID: ${pid}`);
  console.log('========================================');

  // Send SIGINT, similar to Ctrl+C
  const sent = goProcess.kill('SIGINT');

  if (!sent) {
    return res.status(500).json({
      success: false,
      error: 'Failed to stop Go process',
      pid,
    });
  }

  return res.status(200).json({
    success: true,
    message: 'Stop signal sent to Go process',
    pid,
  });
});

// --- Start server ---
app.listen(PORT, () => {
  console.log(`Server running at http://localhost:${PORT}/`);
});
