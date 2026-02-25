# PowerShell script to install Go 1.21 on Windows
# Run as Administrator

Write-Host "================================================" -ForegroundColor Cyan
Write-Host "      Go 1.21 Installation for Windows" -ForegroundColor Cyan
Write-Host "================================================" -ForegroundColor Cyan
Write-Host ""

$goVersion = "go1.21.0.windows-amd64"
$goZip = "$goVersion.zip"
$goUrl = "https://go.dev/dl/$goZip"
$downloadPath = "$env:TEMP\$goZip"
$installPath = "C:\Go"

Write-Host "📥 Step 1: Downloading Go 1.21..." -ForegroundColor Yellow
Write-Host "   URL: $goUrl" -ForegroundColor Gray

try {
    Invoke-WebRequest -Uri $goUrl -OutFile $downloadPath -UseBasicParsing
    Write-Host "   ✅ Downloaded to: $downloadPath" -ForegroundColor Green
} catch {
    Write-Host "   ❌ Download failed: $_" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "📦 Step 2: Extracting Go..." -ForegroundColor Yellow

if (Test-Path $installPath) {
    Write-Host "   ⚠️  Warning: $installPath already exists" -ForegroundColor Yellow
    $confirm = Read-Host "   Overwrite? (y/N)"
    if ($confirm -ne 'y') {
        Write-Host "   Installation cancelled" -ForegroundColor Red
        exit 0
    }
    Remove-Item -Recurse -Force $installPath
}

try {
    Expand-Archive -Path $downloadPath -DestinationPath "C:\" -Force
    Write-Host "   ✅ Extracted to C:\Go" -ForegroundColor Green
} catch {
    Write-Host "   ❌ Extraction failed: $_" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "🔧 Step 3: Configuring Environment Variables..." -ForegroundColor Yellow

# Add Go to PATH for current session
$env:Path += ";C:\Go\bin"

# Add Go to PATH permanently (Machine level)
try {
    [Environment]::SetEnvironmentVariable("Path", $env:Path + ";C:\Go\bin", "Machine")
    Write-Host "   ✅ Added Go to system PATH (restart required for permanent effect)" -ForegroundColor Green
} catch {
    Write-Host "   ⚠️  Warning: Could not modify system PATH (run as Administrator?)" -ForegroundColor Yellow
}

# Set GOPATH and GOROOT
[Environment]::SetEnvironmentVariable("GOROOT", "C:\Go", "User")
[Environment]::SetEnvironmentVariable("GOPATH", "$env:USERPROFILE\go", "User")

Write-Host "   ✅ Set GOROOT = C:\Go" -ForegroundColor Green
Write-Host "   ✅ Set GOPATH = $env:USERPROFILE\go" -ForegroundColor Green

Write-Host ""
Write-Host "🧹 Step 4: Cleaning up..." -ForegroundColor Yellow
Remove-Item $downloadPath -Force
Write-Host "   ✅ Cleanup complete" -ForegroundColor Green

Write-Host ""
Write-Host "✅ Installation Complete!" -ForegroundColor Green
Write-Host ""
Write-Host "================================================" -ForegroundColor Cyan
Write-Host "            Next Steps:" -ForegroundColor Cyan
Write-Host "================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "1. Close this PowerShell window" -ForegroundColor White
Write-Host "2. Open a NEW PowerShell window" -ForegroundColor White
Write-Host "3. Run: go version" -ForegroundColor Yellow
Write-Host "4. If Go works, navigate to CLAWNET:" -ForegroundColor Yellow
Write-Host "   cd C:\Projects\CLAWNET\clawnet" -ForegroundColor Gray
Write-Host ""

Write-Host ""
Write-Host "🎯 Want to test Go now? (y/N)" -ForegroundColor Cyan -NoNewline
$testNow = Read-Host

if ($testNow -eq 'y') {
    Write-Host ""
    Write-Host "Testing Go installation..." -ForegroundColor Yellow

    # Reload environment variables for this session
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";C:\Go\bin"
    $env:GOROOT = "C:\Go"

    $goVersionOutput = & go version 2>&1
    Write-Host $goVersionOutput -ForegroundColor Green
}

Write-Host ""
Write-Host "🚀 Ready to build CLAWNET!" -ForegroundColor Green
Write-Host "   Open a NEW PowerShell window and run:" -ForegroundColor Yellow
Write-Host "   cd C:\Projects\CLAWNET\clawnet" -ForegroundColor Gray
Write-Host "   go mod tidy" -ForegroundColor Gray
Write-Host '   go build -o clawnet.exe cmd/clawnet/main.go' -ForegroundColor Gray
Write-Host ''
