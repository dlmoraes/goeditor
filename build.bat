@echo off
echo =========================================
echo  🚀 Iniciando a compilacao do GoEdit...
echo =========================================
echo.

:: Executa o comando de build otimizado do Go
go build -ldflags="-s -w" -o goedit.exe

:: Verifica se o comando anterior (go build) rodou com sucesso (Error Level 0)
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo ❌ Ocorreu um erro durante a compilacao! Verifique o codigo.
    pause
    exit /b %ERRORLEVEL%
)

echo.
echo ✔️  Sucesso! O ficheiro 'goedit.exe' foi gerado e otimizado.
echo.
pause
