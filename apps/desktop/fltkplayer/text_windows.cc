#include <windows.h>
#include <GL/gl.h>
#include <cstring>
#include <string>
#include <vector>

struct TextBitmap {
    std::string text;
    int size;
    bool symbol;
    int width = 0;
    int height = 0;
    int descent = 0;
    std::vector<unsigned char> pixels;
};

static TextBitmap rasterize(const char* text, int size, bool symbol) {
    TextBitmap result;
    result.text = text;
    result.size = size;
    result.symbol = symbol;
    int count = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, text, -1, nullptr, 0);
    if (count <= 1 || count > 4096) return result;
    std::vector<wchar_t> wide(count);
    if (!MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, text, -1, wide.data(), count)) return result;
    HDC dc = CreateCompatibleDC(nullptr);
    if (!dc) return result;
    HFONT font = CreateFontW(-size, 0, 0, 0, symbol ? FW_NORMAL : FW_BOLD, FALSE, FALSE, FALSE,
        DEFAULT_CHARSET, OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS, NONANTIALIASED_QUALITY,
        DEFAULT_PITCH, symbol ? L"Segoe MDL2 Assets" : L"Microsoft JhengHei UI");
    if (!font) { DeleteDC(dc); return result; }
    HGDIOBJ oldFont = SelectObject(dc, font);
    SIZE extent{};
    TEXTMETRICW metrics{};
    if (GetTextExtentPoint32W(dc, wide.data(), count - 1, &extent) && GetTextMetricsW(dc, &metrics)
        && extent.cx > 0 && extent.cx <= 8192 && metrics.tmHeight > 0 && metrics.tmHeight <= 256) {
        struct MonoBitmapInfo { BITMAPINFOHEADER header; RGBQUAD colors[2]; } info{};
        info.header.biSize = sizeof(BITMAPINFOHEADER);
        info.header.biWidth = extent.cx;
        info.header.biHeight = metrics.tmHeight;
        info.header.biPlanes = 1;
        info.header.biBitCount = 1;
        info.header.biCompression = BI_RGB;
        info.colors[1] = RGBQUAD{255, 255, 255, 0};
        void* bits = nullptr;
        HBITMAP bitmap = CreateDIBSection(dc, reinterpret_cast<BITMAPINFO*>(&info), DIB_RGB_COLORS, &bits, nullptr, 0);
        if (bitmap && bits) {
            HGDIOBJ oldBitmap = SelectObject(dc, bitmap);
            size_t bytes = static_cast<size_t>((extent.cx + 31) / 32) * 4 * metrics.tmHeight;
            std::memset(bits, 0, bytes);
            SetTextColor(dc, RGB(255, 255, 255));
            SetBkColor(dc, RGB(0, 0, 0));
            if (TextOutW(dc, 0, 0, wide.data(), count - 1) && GdiFlush()) {
                result.width = extent.cx;
                result.height = metrics.tmHeight;
                result.descent = metrics.tmDescent;
                result.pixels.assign(static_cast<unsigned char*>(bits), static_cast<unsigned char*>(bits) + bytes);
            }
            SelectObject(dc, oldBitmap);
        }
        if (bitmap) DeleteObject(bitmap);
    }
    SelectObject(dc, oldFont);
    DeleteObject(font);
    DeleteDC(dc);
    return result;
}

void ap_overlay_text(const char* text, int x, int y, int size, bool symbol) {
    static std::vector<TextBitmap> cache;
    const TextBitmap* bitmap = nullptr;
    for (const auto& entry : cache) {
        if (entry.text == text && entry.size == size && entry.symbol == symbol) { bitmap = &entry; break; }
    }
    if (!bitmap) {
        if (cache.size() == 128) cache.clear();
        cache.push_back(rasterize(text, size, symbol));
        bitmap = &cache.back();
    }
    if (bitmap->pixels.empty()) return;
    glPushClientAttrib(GL_CLIENT_PIXEL_STORE_BIT);
    glPixelStorei(GL_UNPACK_ALIGNMENT, 4);
    glPixelStorei(GL_UNPACK_ROW_LENGTH, 0);
    glPixelStorei(GL_UNPACK_SKIP_ROWS, 0);
    glPixelStorei(GL_UNPACK_SKIP_PIXELS, 0);
    glPixelStorei(GL_UNPACK_LSB_FIRST, GL_FALSE);
    glRasterPos2i(x, y);
    glBitmap(bitmap->width, bitmap->height, 0, bitmap->descent, 0, 0, bitmap->pixels.data());
    glPopClientAttrib();
}
