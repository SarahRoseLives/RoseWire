import 'dart:io';
import 'dart:math';
import 'dart:typed_data';
import 'package:flutter/material.dart';
import 'package:path_provider/path_provider.dart';
import 'package:ssh_key/ssh_key.dart' as ssh_key;
import 'package:pointycastle/export.dart' as pointy_castle;
import 'package:dartssh2/dartssh2.dart';

class LoginPanel extends StatefulWidget {
  final void Function(String nickname, String keyPath) onLogin;
  const LoginPanel({super.key, required this.onLogin});

  @override
  State<LoginPanel> createState() => _LoginPanelState();
}

class _LoginPanelState extends State<LoginPanel> {
  bool _loading = true;
  List<Map<String, String>> _keyNickPairs = []; // [{nickname, keyPath}]
  String? _selectedNick;
  String? _selectedKeyPath;
  String _newNick = "";
  bool _creatingNew = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadKeys();
  }

  Future<void> _loadKeys() async {
    final dir = await getApplicationSupportDirectory();
    final keysDir = Directory('${dir.path}/rosewire_keys');
    if (!(await keysDir.exists())) {
      await keysDir.create(recursive: true);
    }
    final keyFiles = keysDir
        .listSync()
        .where((e) => e is File && e.path.endsWith('.pem'))
        .map((e) => e.path)
        .toList();

    List<Map<String, String>> pairs = [];
    for (var keyFile in keyFiles) {
      final nickFile = File(keyFile.replaceAll('.pem', '.nick'));
      if (await nickFile.exists()) {
        final nickname = (await nickFile.readAsString()).trim();
        pairs.add({
          'nickname': nickname,
          'keyPath': keyFile,
        });
      }
    }

    setState(() {
      _keyNickPairs = pairs;
      if (pairs.isNotEmpty && _selectedNick == null) {
        _selectedNick = pairs[0]['nickname'];
        _selectedKeyPath = pairs[0]['keyPath'];
      }
      _loading = false;
    });
  }

  Future<void> _createNewKeyAndNick() async {
    if (_newNick.trim().isEmpty) {
      setState(() => _error = "Nickname required.");
      return;
    }
    setState(() {
      _loading = true;
      _error = null;
    });

    try {
      final keyGen = pointy_castle.RSAKeyGenerator();
      final random = pointy_castle.FortunaRandom();
      final seed = Uint8List.fromList(
          List<int>.generate(32, (_) => Random.secure().nextInt(256)));
      random.seed(pointy_castle.KeyParameter(seed));
      keyGen.init(pointy_castle.ParametersWithRandom(
          pointy_castle.RSAKeyGeneratorParameters(
              BigInt.from(65537), 2048, 64),
          random));

      final keyPair = keyGen.generateKeyPair();
      final rsaPublicKey = keyPair.publicKey as pointy_castle.RSAPublicKey;
      final rsaPrivateKey = keyPair.privateKey as pointy_castle.RSAPrivateKey;

      final pubKeyWithInfo =
          ssh_key.RSAPublicKeyWithInfo.fromRSAPublicKey(rsaPublicKey);
      final pvtKeyWithInfo =
          ssh_key.RSAPrivateKeyWithInfo.fromRSAPrivateKey(rsaPrivateKey);

      final privateKeyPem = pvtKeyWithInfo.encode(ssh_key.PvtKeyEncoding.pkcs1);
      final publicKeyOpenSsh =
          pubKeyWithInfo.encode(ssh_key.PubKeyEncoding.openSsh);

      final dir = await getApplicationSupportDirectory();
      final keysDir = Directory('${dir.path}/rosewire_keys');
      if (!(await keysDir.exists())) {
        await keysDir.create(recursive: true);
      }
      final filenameBase =
          '${DateTime.now().millisecondsSinceEpoch}_${_newNick.trim().replaceAll(" ", "_")}';
      final keyPath = '${keysDir.path}/$filenameBase.pem';
      final pubPath = keyPath.replaceAll('.pem', '.pub');
      final nickPath = keyPath.replaceAll('.pem', '.nick');

      await File(keyPath).writeAsString(privateKeyPem);
      await File(pubPath).writeAsString(publicKeyOpenSsh);
      await File(nickPath).writeAsString(_newNick.trim());

      setState(() {
        _selectedNick = _newNick.trim();
        _selectedKeyPath = keyPath;
        _creatingNew = false;
        _newNick = "";
      });
      await _loadKeys();
      await _onLoginPressed();
    } catch (e) {
      setState(() {
        _error = "Error creating key: $e";
        _loading = false;
      });
    }
  }

  Future<void> _onLoginPressed() async {
    if (_selectedNick == null || _selectedKeyPath == null) {
      setState(() => _error = "Please select or create a profile.");
      return;
    }

    setState(() {
      _loading = true;
      _error = null;
    });

    SSHClient? client;
    try {
      final privateKey = await File(_selectedKeyPath!).readAsString();

      // Changed from 'localhost' to 'sarahsforge.dev'
      final socket = await SSHSocket.connect('sarahsforge.dev', 2222);

      client = SSHClient(
        socket,
        username: _selectedNick!,
        identities: SSHKeyPair.fromPem(privateKey),
      );

      await client.authenticated;

      client.close();
      await client.done;

      widget.onLogin(_selectedNick!, _selectedKeyPath!);
    } catch (e) {
      setState(() {
        _error = "Login failed: ${e.toString()}";
        _loading = false;
      });
      if (client != null) {
        client.close();
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      body: Center(
        child: Card(
          color: Colors.grey[900],
          elevation: 12,
          shape:
              RoundedRectangleBorder(borderRadius: BorderRadius.circular(24)),
          child: Container(
            width: 400,
            padding: const EdgeInsets.all(32),
            child: _loading
                ? Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      CircularProgressIndicator(),
                      SizedBox(height: 16),
                      Text("Connecting...", style: TextStyle(color: Colors.white70))
                    ],
                  )
                : Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const Text(
                        "RoseWire Login",
                        style: TextStyle(
                            fontSize: 26,
                            fontWeight: FontWeight.bold,
                            color: Colors.pinkAccent),
                      ),
                      const SizedBox(height: 24),
                      if (_keyNickPairs.isNotEmpty && !_creatingNew) ...[
                        DropdownButtonFormField<String>(
                          value: _selectedNick,
                          items: _keyNickPairs
                              .map((pair) => DropdownMenuItem(
                                    value: pair['nickname'],
                                    child: Text(pair['nickname']!),
                                  ))
                              .toList(),
                          onChanged: _loading ? null : (val) {
                            final selected = _keyNickPairs
                                .firstWhere((pair) => pair['nickname'] == val);
                            setState(() {
                              _selectedNick = val;
                              _selectedKeyPath = selected['keyPath'];
                            });
                          },
                          decoration:
                              const InputDecoration(labelText: "Select Profile"),
                        ),
                        const SizedBox(height: 16),
                        ElevatedButton.icon(
                          icon: const Icon(Icons.login),
                          label: const Text("Login"),
                          onPressed: _loading ? null : _onLoginPressed,
                        ),
                        const SizedBox(height: 16),
                        TextButton(
                          child: const Text("Create new profile"),
                          onPressed: _loading ? null : () => setState(() => _creatingNew = true),
                        ),
                      ],
                      if (_creatingNew || _keyNickPairs.isEmpty) ...[
                        TextField(
                          decoration: const InputDecoration(
                              labelText: "Enter new nickname"),
                          onChanged: (val) => _newNick = val,
                        ),
                        const SizedBox(height: 16),
                        ElevatedButton.icon(
                          icon: const Icon(Icons.vpn_key),
                          label: const Text("Generate & Login"),
                          onPressed: _loading ? null : _createNewKeyAndNick,
                        ),
                        if (_keyNickPairs.isNotEmpty)
                          TextButton(
                            child: const Text("Back"),
                            onPressed: _loading ? null : () => setState(() => _creatingNew = false),
                          ),
                      ],
                      if (_error != null)
                        Padding(
                          padding: const EdgeInsets.only(top: 10),
                          child:
                              Text(_error!, style: TextStyle(color: Colors.redAccent)),
                        ),
                    ],
                  ),
          ),
        ),
      ),
    );
  }
}