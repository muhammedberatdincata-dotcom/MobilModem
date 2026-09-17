using System;
using System.Diagnostics;
using System.IO;
using System.Linq;

namespace MobilModemClient;

public class AdbManager
{
    private string? _adbPath;
    private bool _isForwarding;

    public bool IsForwarding => _isForwarding;
    public string? AdbPath => _adbPath;

    public string? FindAdb()
    {
        if (!string.IsNullOrEmpty(_adbPath) && File.Exists(_adbPath))
            return _adbPath;

        // 1. Check Temp extraction folder
        string tempAdb = Path.Combine(Path.GetTempPath(), "MobilModem", "adb.exe");
        if (File.Exists(tempAdb))
        {
            _adbPath = tempAdb;
            return _adbPath;
        }

        // 2. Check App Directory
        string appDirAdb = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "adb.exe");
        if (File.Exists(appDirAdb))
        {
            _adbPath = appDirAdb;
            return _adbPath;
        }

        // 3. Check system PATH
        var pathEnv = Environment.GetEnvironmentVariable("PATH");
        if (!string.IsNullOrEmpty(pathEnv))
        {
            foreach (var dir in pathEnv.Split(Path.PathSeparator, StringSplitOptions.RemoveEmptyEntries))
            {
                try
                {
                    string candidate = Path.Combine(dir.Trim(), "adb.exe");
                    if (File.Exists(candidate))
                    {
                        _adbPath = candidate;
                        return _adbPath;
                    }
                }
                catch { }
            }
        }

        return null;
    }

    public (bool isConnected, string? deviceSerial) CheckDevice()
    {
        var adb = FindAdb();
        if (string.IsNullOrEmpty(adb))
            return (false, null);

        try
        {
            var psi = new ProcessStartInfo
            {
                FileName = adb,
                Arguments = "devices",
                RedirectStandardOutput = true,
                UseShellExecute = false,
                CreateNoWindow = true
            };
            using var proc = Process.Start(psi);
            if (proc == null) return (false, null);

            string output = proc.StandardOutput.ReadToEnd();
            proc.WaitForExit();

            using var reader = new StringReader(output);
            string? line;
            while ((line = reader.ReadLine()) != null)
            {
                line = line.Trim();
                if (line.EndsWith("device") && !line.StartsWith("List of devices", StringComparison.OrdinalIgnoreCase))
                {
                    var parts = line.Split('\t', StringSplitOptions.RemoveEmptyEntries);
                    string serial = parts.Length > 0 ? parts[0] : "Cihaz";
                    return (true, serial);
                }
            }
        }
        catch { }

        return (false, null);
    }

    public bool SetupForwarding(out string? error)
    {
        error = null;
        var adb = FindAdb();
        if (string.IsNullOrEmpty(adb))
        {
            error = "adb.exe bulunamadı.";
            return false;
        }

        try
        {
            var psi = new ProcessStartInfo
            {
                FileName = adb,
                Arguments = "forward tcp:10808 tcp:10808",
                RedirectStandardError = true,
                UseShellExecute = false,
                CreateNoWindow = true
            };
            using var proc = Process.Start(psi);
            if (proc == null)
            {
                error = "ADB başlatılamadı.";
                return false;
            }

            string err = proc.StandardError.ReadToEnd();
            proc.WaitForExit();

            if (proc.ExitCode != 0)
            {
                error = $"adb forward başarısız: {err}";
                return false;
            }

            _isForwarding = true;
            return true;
        }
        catch (Exception ex)
        {
            error = ex.Message;
            return false;
        }
    }

    public void RemoveForwarding()
    {
        if (!_isForwarding) return;
        var adb = FindAdb();
        if (string.IsNullOrEmpty(adb)) return;

        try
        {
            var psi = new ProcessStartInfo
            {
                FileName = adb,
                Arguments = "forward --remove tcp:10808",
                UseShellExecute = false,
                CreateNoWindow = true
            };
            using var proc = Process.Start(psi);
            proc?.WaitForExit();
        }
        catch { }
        finally
        {
            _isForwarding = false;
        }
    }

    public bool InstallApkToDevice(string apkPath, out string output, out string? error)
    {
        output = "";
        error = null;

        var adb = FindAdb();
        if (string.IsNullOrEmpty(adb))
        {
            error = "ADB bulunamadı. Lütfen adb.exe'nin mevcut olduğundan emin olun.";
            return false;
        }

        if (!File.Exists(apkPath))
        {
            error = $"APK dosyası bulunamadı: {apkPath}";
            return false;
        }

        try
        {
            var psi = new ProcessStartInfo
            {
                FileName = adb,
                Arguments = $"install -r \"{apkPath}\"",
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                UseShellExecute = false,
                CreateNoWindow = true
            };
            using var proc = Process.Start(psi);
            if (proc == null)
            {
                error = "Yükleme işlemi başlatılamadı.";
                return false;
            }

            output = proc.StandardOutput.ReadToEnd();
            string err = proc.StandardError.ReadToEnd();
            proc.WaitForExit();

            if (proc.ExitCode != 0 || !output.Contains("Success", StringComparison.OrdinalIgnoreCase))
            {
                error = $"Yükleme başarısız: {err} {output}";
                return false;
            }

            return true;
        }
        catch (Exception ex)
        {
            error = ex.Message;
            return false;
        }
    }
}
