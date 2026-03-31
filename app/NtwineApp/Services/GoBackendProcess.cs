using System;
using System.Diagnostics;
using System.IO;
using System.Net.Http;
using System.Threading.Tasks;

namespace NtwineApp.Services;

public class GoBackendProcess : IDisposable
{
    private Process? _process;
    private readonly string _port;
    private bool _running;

    public string BaseUrl => $"http://localhost:{_port}";
    public string WsUrl => $"ws://localhost:{_port}/api/discuss";
    public bool IsRunning => _running;

    public GoBackendProcess(string port = "8090")
    {
        _port = port;
    }

    public async Task<bool> Start(string workingDir = ".")
    {
        if (_running) return true;

        var goBinary = FindGoBinary();
        if (goBinary == null)
        {
            Console.WriteLine("[backend] go binary not found, trying to use existing server on port " + _port);
            return await CheckHealth();
        }

        try
        {
            var serverDir = FindServerDir();
            if (serverDir == null)
            {
                Console.WriteLine("[backend] server source not found");
                return await CheckHealth();
            }

            _process = new Process
            {
                StartInfo = new ProcessStartInfo
                {
                    FileName = goBinary,
                    Arguments = $"run ./cmd/server -port {_port} -no-browser",
                    WorkingDirectory = serverDir,
                    UseShellExecute = false,
                    RedirectStandardOutput = true,
                    RedirectStandardError = true,
                    CreateNoWindow = true
                }
            };

            _process.OutputDataReceived += (_, e) =>
            {
                if (e.Data != null) Console.WriteLine($"[backend] {e.Data}");
            };
            _process.ErrorDataReceived += (_, e) =>
            {
                if (e.Data != null) Console.WriteLine($"[backend:err] {e.Data}");
            };

            _process.Start();
            _process.BeginOutputReadLine();
            _process.BeginErrorReadLine();

            for (int i = 0; i < 30; i++)
            {
                await Task.Delay(500);
                if (await CheckHealth())
                {
                    _running = true;
                    Console.WriteLine($"[backend] started on port {_port}");
                    return true;
                }
            }

            Console.WriteLine("[backend] timed out waiting for server");
            return false;
        }
        catch (Exception ex)
        {
            Console.WriteLine($"[backend] failed to start: {ex.Message}");
            return await CheckHealth();
        }
    }

    public void Stop()
    {
        if (_process != null && !_process.HasExited)
        {
            try
            {
                _process.Kill(entireProcessTree: true);
            }
            catch { }
        }
        _running = false;
    }

    public void Dispose()
    {
        Stop();
        _process?.Dispose();
    }

    private async Task<bool> CheckHealth()
    {
        try
        {
            using var http = new HttpClient { Timeout = TimeSpan.FromSeconds(2) };
            var resp = await http.GetAsync($"{BaseUrl}/api/config");
            _running = resp.IsSuccessStatusCode;
            return _running;
        }
        catch
        {
            return false;
        }
    }

    private static string? FindGoBinary()
    {
        var paths = new[]
        {
            "/usr/local/go/bin/go",
            "/opt/homebrew/bin/go",
            "/usr/bin/go",
        };

        foreach (var p in paths)
        {
            if (File.Exists(p)) return p;
        }

        try
        {
            var proc = Process.Start(new ProcessStartInfo
            {
                FileName = "which",
                Arguments = "go",
                RedirectStandardOutput = true,
                UseShellExecute = false,
                CreateNoWindow = true
            });
            proc?.WaitForExit(3000);
            var path = proc?.StandardOutput.ReadToEnd().Trim();
            if (!string.IsNullOrEmpty(path) && File.Exists(path)) return path;
        }
        catch { }

        return null;
    }

    private static string? FindServerDir()
    {
        var candidates = new[]
        {
            Path.Combine(AppContext.BaseDirectory, "..", "..", "..", ".."),
            Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), "Documents", "ntwine"),
            "."
        };

        foreach (var dir in candidates)
        {
            var full = Path.GetFullPath(dir);
            if (File.Exists(Path.Combine(full, "cmd", "server", "main.go")))
                return full;
        }

        return null;
    }
}
