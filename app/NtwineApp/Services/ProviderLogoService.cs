using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.IO;
using System.Net.Http;
using System.Threading.Tasks;
using Avalonia.Media.Imaging;

namespace NtwineApp.Services;

public static class ProviderLogoService
{
    private static readonly HttpClient Http = new() { Timeout = TimeSpan.FromSeconds(5) };
    private static readonly ConcurrentDictionary<string, Bitmap?> Cache = new();

    private static readonly string CacheDir = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".ntwine", "logos");

    private static readonly Dictionary<string, string> ProviderDomains = new()
    {
        ["anthropic"] = "anthropic.com",
        ["openai"] = "openai.com",
        ["google"] = "google.com",
        ["deepseek"] = "deepseek.com",
        ["meta-llama"] = "meta.com",
        ["mistralai"] = "mistral.ai",
        ["qwen"] = "qwenlm.com",
        ["x-ai"] = "x.ai",
        ["cohere"] = "cohere.com",
        ["microsoft"] = "microsoft.com",
        ["amazon"] = "amazon.com",
        ["minimax"] = "minimax.io",
        ["nousresearch"] = "nousresearch.com",
        ["moonshotai"] = "moonshot.ai",
    };

    public static string GetProviderColor(string slug)
    {
        return slug switch
        {
            "anthropic" => "#d4a27f",
            "openai" => "#10b981",
            "google" => "#4285f4",
            "deepseek" => "#06b6d4",
            "meta-llama" => "#0668E1",
            "mistralai" => "#f97316",
            "qwen" => "#6366f1",
            "x-ai" => "#ef4444",
            "cohere" => "#39594d",
            "microsoft" => "#00a4ef",
            "amazon" => "#ff9900",
            "minimax" => "#ec4899",
            _ => "#7a6f96"
        };
    }

    public static async Task<Bitmap?> LoadLogo(string providerSlug)
    {
        if (Cache.TryGetValue(providerSlug, out var cached))
            return cached;

        Directory.CreateDirectory(CacheDir);
        var diskPath = Path.Combine(CacheDir, $"{providerSlug}.png");

        if (File.Exists(diskPath))
        {
            try
            {
                var bmp = new Bitmap(diskPath);
                Cache[providerSlug] = bmp;
                return bmp;
            }
            catch { }
        }

        if (!ProviderDomains.TryGetValue(providerSlug, out var domain))
        {
            Cache[providerSlug] = null;
            return null;
        }

        try
        {
            var bytes = await Http.GetByteArrayAsync($"https://www.google.com/s2/favicons?domain={domain}&sz=128");
            await File.WriteAllBytesAsync(diskPath, bytes);

            using var stream = new MemoryStream(bytes);
            var bmp = new Bitmap(stream);
            Cache[providerSlug] = bmp;
            return bmp;
        }
        catch
        {
            Cache[providerSlug] = null;
            return null;
        }
    }
}
