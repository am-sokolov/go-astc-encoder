// SPDX-License-Identifier: Apache-2.0
//
// Minimal in-memory benchmark harness for astcenc.
//
// Build example (Linux/macOS):
//   c++ -O3 -std=c++14 -I Source bench/astcbenchcpp.cpp build/Source/libastcenc-avx2-static.a -o astcbenchcpp -pthread
//
// This binary is not part of the library API; it's used for A/B performance comparisons against the Go port.

#include <chrono>
#include <cstdint>
#include <cstring>
#include <fstream>
#include <iostream>
#include <string>
#include <vector>

#include "astcenc.h"

static void usage() {
    std::cerr << "usage:\n";
    std::cerr << "  astcbenchcpp decode -in <file.astc> [-profile ldr|srgb|hdr|hdr-rgb-ldr-a] [-iters N] [-out u8|f32] [-checksum fnv|none]\n";
    std::cerr << "  astcbenchcpp encode -w W -h H [-d D] -block 4x4[ xZ] [-profile ldr|srgb|hdr|hdr-rgb-ldr-a|hdr] "
                 "[-quality fastest|fast|medium|thorough|verythorough|exhaustive] [-iters N] [-out file.astc] [-checksum fnv|none]\n";
}

static bool starts_with(const std::string& s, const char* pfx) {
    return s.rfind(pfx, 0) == 0;
}

static bool get_flag_value(const std::vector<std::string>& args, const char* key, std::string& out) {
    for (size_t i = 0; i + 1 < args.size(); i++) {
        if (args[i] == key) {
            out = args[i + 1];
            return true;
        }
    }
    return false;
}

static int get_flag_int(const std::vector<std::string>& args, const char* key, int def) {
    std::string v;
    if (!get_flag_value(args, key, v)) {
        return def;
    }
    return std::stoi(v);
}

static std::string get_flag_string(const std::vector<std::string>& args, const char* key, const char* def) {
    std::string v;
    if (!get_flag_value(args, key, v)) {
        return def;
    }
    return v;
}

static bool has_flag(const std::vector<std::string>& args, const char* key) {
    for (auto& a : args) {
        if (a == key) {
            return true;
        }
    }
    return false;
}

static astcenc_profile parse_profile(const std::string& s) {
    std::string v = s;
    for (auto& c : v) c = static_cast<char>(std::tolower(c));
    if (v == "ldr") return ASTCENC_PRF_LDR;
    if (v == "srgb" || v == "ldr-srgb") return ASTCENC_PRF_LDR_SRGB;
    if (v == "hdr" || v == "hdr-rgba") return ASTCENC_PRF_HDR;
    if (v == "hdr-rgb-ldr-a" || v == "hdr-rgb-ldr-alpha") return ASTCENC_PRF_HDR_RGB_LDR_A;
    throw std::runtime_error("invalid -profile");
}

static float parse_quality(const std::string& s) {
    std::string v = s;
    for (auto& c : v) c = static_cast<char>(std::tolower(c));
    if (v == "fastest") return ASTCENC_PRE_FASTEST;
    if (v == "fast") return ASTCENC_PRE_FAST;
    if (v == "medium") return ASTCENC_PRE_MEDIUM;
    if (v == "thorough") return ASTCENC_PRE_THOROUGH;
    if (v == "verythorough" || v == "very-thorough") return ASTCENC_PRE_VERYTHOROUGH;
    if (v == "exhaustive") return ASTCENC_PRE_EXHAUSTIVE;
    throw std::runtime_error("invalid -quality");
}

static void parse_block(const std::string& s, int& bx, int& by, int& bz) {
    bx = by = bz = 1;
    int n = 0;
    for (auto c : s) if (c == 'x') n++;
    if (n == 1) {
        if (std::sscanf(s.c_str(), "%dx%d", &bx, &by) != 2) throw std::runtime_error("invalid -block");
        bz = 1;
        return;
    }
    if (n == 2) {
        if (std::sscanf(s.c_str(), "%dx%dx%d", &bx, &by, &bz) != 3) throw std::runtime_error("invalid -block");
        return;
    }
    throw std::runtime_error("invalid -block");
}

static uint64_t fnv1a64(uint64_t seed, const uint8_t* data, size_t len) {
    const uint64_t offset64 = 14695981039346656037ull;
    const uint64_t prime64 = 1099511628211ull;
    uint64_t h = seed ? seed : offset64;
    for (size_t i = 0; i < len; i++) {
        h ^= data[i];
        h *= prime64;
    }
    return h;
}

static std::string hex_u64(uint64_t v) {
    static const char* hexd = "0123456789abcdef";
    std::string out(16, '0');
    for (int i = 0; i < 8; i++) {
        uint8_t b = static_cast<uint8_t>(v >> (56 - i * 8));
        out[i * 2 + 0] = hexd[b >> 4];
        out[i * 2 + 1] = hexd[b & 0xF];
    }
    return out;
}

static void fill_pattern_rgba8(uint8_t* pix, int width, int height, int depth) {
    for (int z = 0; z < depth; z++) {
        for (int y = 0; y < height; y++) {
            for (int x = 0; x < width; x++) {
                size_t off = static_cast<size_t>(((z * height + y) * width + x) * 4);
                uint32_t r = static_cast<uint32_t>(x * 3 + y * 5 + z * 7);
                uint32_t g = static_cast<uint32_t>(x * 11 + y * 13 + z * 17);
                uint32_t b = static_cast<uint32_t>(x ^ y ^ z);
                uint32_t a = 255u - static_cast<uint32_t>((x * 5 + y * 7 + z * 3) & 0xFF);
                pix[off + 0] = static_cast<uint8_t>(r);
                pix[off + 1] = static_cast<uint8_t>(g);
                pix[off + 2] = static_cast<uint8_t>(b);
                pix[off + 3] = static_cast<uint8_t>(a);
            }
        }
    }
}

static uint32_t read_u24_le(const uint8_t* p) {
    return static_cast<uint32_t>(p[0]) | (static_cast<uint32_t>(p[1]) << 8) | (static_cast<uint32_t>(p[2]) << 16);
}

static void write_u24_le(uint8_t* p, uint32_t v) {
    p[0] = static_cast<uint8_t>(v);
    p[1] = static_cast<uint8_t>(v >> 8);
    p[2] = static_cast<uint8_t>(v >> 16);
}

static void write_astc_header(uint8_t* out16, int bx, int by, int bz, int sx, int sy, int sz) {
    out16[0] = 0x13;
    out16[1] = 0xAB;
    out16[2] = 0xA1;
    out16[3] = 0x5C;
    out16[4] = static_cast<uint8_t>(bx);
    out16[5] = static_cast<uint8_t>(by);
    out16[6] = static_cast<uint8_t>(bz);
    write_u24_le(out16 + 7, static_cast<uint32_t>(sx));
    write_u24_le(out16 + 10, static_cast<uint32_t>(sy));
    write_u24_le(out16 + 13, static_cast<uint32_t>(sz));
}

static void decode_main(const std::vector<std::string>& args) {
    std::string in_path = get_flag_string(args, "-in", "");
    if (in_path.empty()) {
        throw std::runtime_error("missing -in");
    }

    std::string prof_str = get_flag_string(args, "-profile", "ldr");
    std::string out_kind = get_flag_string(args, "-out", "u8");
    std::string checksum_opt = get_flag_string(args, "-checksum", "fnv");
    int iters = get_flag_int(args, "-iters", 200);
    if (iters <= 0) {
        throw std::runtime_error("iters must be > 0");
    }

    astcenc_profile profile = parse_profile(prof_str);
    bool do_checksum = true;
    for (auto& c : checksum_opt) c = static_cast<char>(std::tolower(c));
    if (checksum_opt == "none") do_checksum = false;

    // Read file.
    std::ifstream f(in_path, std::ios::binary);
    if (!f) {
        throw std::runtime_error("failed to open input");
    }
    std::vector<uint8_t> file((std::istreambuf_iterator<char>(f)), std::istreambuf_iterator<char>());
    if (file.size() < 16) {
        throw std::runtime_error("input too small");
    }
    const uint8_t* hdr = file.data();
    if (hdr[0] != 0x13 || hdr[1] != 0xAB || hdr[2] != 0xA1 || hdr[3] != 0x5C) {
        throw std::runtime_error("invalid ASTC magic");
    }

    int bx = hdr[4];
    int by = hdr[5];
    int bz = hdr[6];
    int sx = static_cast<int>(read_u24_le(hdr + 7));
    int sy = static_cast<int>(read_u24_le(hdr + 10));
    int sz = static_cast<int>(read_u24_le(hdr + 13));
    if (bx <= 0 || by <= 0 || bz <= 0 || sx <= 0 || sy <= 0 || sz <= 0) {
        throw std::runtime_error("invalid ASTC header");
    }

    int blocks_x = (sx + bx - 1) / bx;
    int blocks_y = (sy + by - 1) / by;
    int blocks_z = (sz + bz - 1) / bz;
    size_t total_blocks = static_cast<size_t>(blocks_x) * static_cast<size_t>(blocks_y) * static_cast<size_t>(blocks_z);
    size_t need = 16 + total_blocks * 16;
    if (file.size() < need) {
        throw std::runtime_error("truncated ASTC data");
    }
    const uint8_t* blocks = file.data() + 16;
    size_t blocks_len = total_blocks * 16;

    // Setup context.
    astcenc_config cfg {};
    unsigned int flags = ASTCENC_FLG_DECOMPRESS_ONLY;
    astcenc_error e = astcenc_config_init(profile, bx, by, bz, ASTCENC_PRE_FASTEST, flags, &cfg);
    if (e != ASTCENC_SUCCESS) {
        throw std::runtime_error("astcenc_config_init failed");
    }

    astcenc_context* ctx = nullptr;
    e = astcenc_context_alloc(&cfg, 1, &ctx);
    if (e != ASTCENC_SUCCESS || !ctx) {
        throw std::runtime_error("astcenc_context_alloc failed");
    }

    astcenc_swizzle swz { ASTCENC_SWZ_R, ASTCENC_SWZ_G, ASTCENC_SWZ_B, ASTCENC_SWZ_A };

    uint64_t checksum = 0;
    auto t0 = std::chrono::steady_clock::now();

    if (out_kind == "u8" || out_kind == "rgba8") {
        std::vector<uint8_t> out(static_cast<size_t>(sx) * static_cast<size_t>(sy) * static_cast<size_t>(sz) * 4);
        std::vector<void*> slices(static_cast<size_t>(sz));
        for (int z = 0; z < sz; z++) {
            slices[static_cast<size_t>(z)] = out.data() + static_cast<size_t>(z) * static_cast<size_t>(sx) * static_cast<size_t>(sy) * 4;
        }
        astcenc_image img { static_cast<unsigned int>(sx), static_cast<unsigned int>(sy), static_cast<unsigned int>(sz), ASTCENC_TYPE_U8, slices.data() };

        for (int i = 0; i < iters; i++) {
            e = astcenc_decompress_image(ctx, blocks, blocks_len, &img, &swz, 0);
            if (e != ASTCENC_SUCCESS) {
                throw std::runtime_error("astcenc_decompress_image failed");
            }
            if (do_checksum) {
                checksum = fnv1a64(checksum, out.data(), out.size());
            }
        }
    } else if (out_kind == "f32") {
        std::vector<float> out(static_cast<size_t>(sx) * static_cast<size_t>(sy) * static_cast<size_t>(sz) * 4);
        std::vector<void*> slices(static_cast<size_t>(sz));
        for (int z = 0; z < sz; z++) {
            slices[static_cast<size_t>(z)] = out.data() + static_cast<size_t>(z) * static_cast<size_t>(sx) * static_cast<size_t>(sy) * 4;
        }
        astcenc_image img { static_cast<unsigned int>(sx), static_cast<unsigned int>(sy), static_cast<unsigned int>(sz), ASTCENC_TYPE_F32, slices.data() };

        for (int i = 0; i < iters; i++) {
            e = astcenc_decompress_image(ctx, blocks, blocks_len, &img, &swz, 0);
            if (e != ASTCENC_SUCCESS) {
                throw std::runtime_error("astcenc_decompress_image failed");
            }
            if (do_checksum) {
                checksum = fnv1a64(checksum, reinterpret_cast<const uint8_t*>(out.data()), out.size() * sizeof(float));
            }
        }
    } else {
        throw std::runtime_error("invalid -out (want u8|f32)");
    }

    auto t1 = std::chrono::steady_clock::now();
    std::chrono::duration<double> dur = t1 - t0;

    double texels = static_cast<double>(sx) * static_cast<double>(sy) * static_cast<double>(sz) * static_cast<double>(iters);
    double mpix_s = texels / dur.count() / 1e6;

    std::cout << "RESULT impl=cpp mode=decode out=" << out_kind
              << " profile=" << prof_str
              << " size=" << sx << "x" << sy << "x" << sz
              << " iters=" << iters
              << " seconds=" << dur.count()
              << " mpix/s=" << mpix_s
              << " checksum=" << (do_checksum ? hex_u64(checksum) : std::string("none")) << "\n";

    astcenc_context_free(ctx);
}

static void encode_main(const std::vector<std::string>& args) {
    int w = get_flag_int(args, "-w", 256);
    int h = get_flag_int(args, "-h", 256);
    int d = get_flag_int(args, "-d", 1);
    int iters = get_flag_int(args, "-iters", 20);
    if (w <= 0 || h <= 0 || d <= 0 || iters <= 0) {
        throw std::runtime_error("invalid dimensions/iters");
    }

    std::string block_str = get_flag_string(args, "-block", "4x4");
    int bx = 0, by = 0, bz = 0;
    parse_block(block_str, bx, by, bz);

    std::string prof_str = get_flag_string(args, "-profile", "ldr");
    std::string quality_str = get_flag_string(args, "-quality", "medium");
    std::string out_path = get_flag_string(args, "-out", "");
    std::string checksum_opt = get_flag_string(args, "-checksum", "fnv");

    astcenc_profile profile = parse_profile(prof_str);
    float quality = parse_quality(quality_str);
    bool do_checksum = true;
    for (auto& c : checksum_opt) c = static_cast<char>(std::tolower(c));
    if (checksum_opt == "none") do_checksum = false;

    // Generate synthetic input.
    std::vector<uint8_t> pix(static_cast<size_t>(w) * static_cast<size_t>(h) * static_cast<size_t>(d) * 4);
    fill_pattern_rgba8(pix.data(), w, h, d);

    std::vector<void*> slices(static_cast<size_t>(d));
    for (int z = 0; z < d; z++) {
        slices[static_cast<size_t>(z)] = pix.data() + static_cast<size_t>(z) * static_cast<size_t>(w) * static_cast<size_t>(h) * 4;
    }
    astcenc_image img { static_cast<unsigned int>(w), static_cast<unsigned int>(h), static_cast<unsigned int>(d), ASTCENC_TYPE_U8, slices.data() };
    astcenc_swizzle swz { ASTCENC_SWZ_R, ASTCENC_SWZ_G, ASTCENC_SWZ_B, ASTCENC_SWZ_A };

    // Context.
    astcenc_config cfg {};
    unsigned int flags = 0;
    astcenc_error e = astcenc_config_init(profile, bx, by, bz, quality, flags, &cfg);
    if (e != ASTCENC_SUCCESS) {
        throw std::runtime_error("astcenc_config_init failed");
    }

    astcenc_context* ctx = nullptr;
    e = astcenc_context_alloc(&cfg, 1, &ctx);
    if (e != ASTCENC_SUCCESS || !ctx) {
        throw std::runtime_error("astcenc_context_alloc failed");
    }

    int blocks_x = (w + bx - 1) / bx;
    int blocks_y = (h + by - 1) / by;
    int blocks_z = (d + bz - 1) / bz;
    size_t total_blocks = static_cast<size_t>(blocks_x) * static_cast<size_t>(blocks_y) * static_cast<size_t>(blocks_z);
    size_t out_len = total_blocks * 16;

    uint64_t checksum = 0;
    std::vector<uint8_t> out(out_len);

    auto t0 = std::chrono::steady_clock::now();
    for (int i = 0; i < iters; i++) {
        std::memset(out.data(), 0, out.size());
        e = astcenc_compress_image(ctx, &img, &swz, out.data(), out.size(), 0);
        if (e != ASTCENC_SUCCESS) {
            throw std::runtime_error("astcenc_compress_image failed");
        }
        if (do_checksum) {
            checksum = fnv1a64(checksum, out.data(), out.size());
        }
        astcenc_compress_reset(ctx);
    }
    auto t1 = std::chrono::steady_clock::now();
    std::chrono::duration<double> dur = t1 - t0;

    if (!out_path.empty()) {
        std::vector<uint8_t> file(16 + out.size());
        write_astc_header(file.data(), bx, by, bz, w, h, d);
        std::memcpy(file.data() + 16, out.data(), out.size());
        std::ofstream f(out_path, std::ios::binary);
        f.write(reinterpret_cast<const char*>(file.data()), static_cast<std::streamsize>(file.size()));
    }

    double texels = static_cast<double>(w) * static_cast<double>(h) * static_cast<double>(d) * static_cast<double>(iters);
    double mpix_s = texels / dur.count() / 1e6;

    std::cout << "RESULT impl=cpp mode=encode"
              << " profile=" << prof_str
              << " block=" << block_str
              << " size=" << w << "x" << h << "x" << d
              << " iters=" << iters
              << " seconds=" << dur.count()
              << " mpix/s=" << mpix_s
              << " checksum=" << (do_checksum ? hex_u64(checksum) : std::string("none")) << "\n";

    astcenc_context_free(ctx);
}

int main(int argc, char** argv) {
    try {
        if (argc < 2) {
            usage();
            return 2;
        }

        std::string cmd = argv[1];
        std::vector<std::string> args;
        for (int i = 2; i < argc; i++) {
            args.emplace_back(argv[i]);
        }

        if (cmd == "decode") {
            decode_main(args);
            return 0;
        }
        if (cmd == "encode") {
            encode_main(args);
            return 0;
        }

        usage();
        return 2;
    } catch (const std::exception& e) {
        std::cerr << "error: " << e.what() << "\n";
        return 1;
    }
}
