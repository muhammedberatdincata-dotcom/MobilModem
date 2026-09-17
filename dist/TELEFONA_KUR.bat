@echo off
chcp 65001 > nul
echo ========================================================
echo   MobilModem APK - Telefona Otomatik Kurulum
echo ========================================================
echo.
echo [1/2] ADB Cihazi kontrol ediliyor...
adb devices
echo.
echo [2/2] MobilModem.apk yukleniyor...
adb install -r MobilModem.apk
if %errorlevel% equ 0 (
    echo.
    echo ========================================================
    echo   TEBRIKLER! MobilModem basariyla telefonunuza kuruldu!
    echo ========================================================
) else (
    echo.
    echo [HATA] Yukleme basarisiz! Lutfen USB Hata Ayiklamayi acip kabloyu tekrar takin.
)
echo.
pause
