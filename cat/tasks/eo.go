package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: eo <file.eo>")
		return
	}

	filename := os.Args[1]
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer file.Close()

	importPaths := map[string]string{
		"timeo":  "\"time\"",
		"matheo": "\"math\"",
		"randeo": "\"math/rand\"",
		"oseo":   "\"os\"",
	}

	importPkgNames := map[string]string{
		"timeo":  "time",
		"matheo": "math",
		"randeo": "rand",
		"oseo":   "os",
	}

	aliases := make(map[string]string)
	activeLibRenames := make(map[string]string)
	var translatedImports []string
	var translatedLines []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "import ") {
			libName := strings.TrimSpace(strings.TrimPrefix(line, "import"))
			if realPath, exists := importPaths[libName]; exists {
				translatedImports = append(translatedImports, realPath)
				activeLibRenames[libName] = importPkgNames[libName]
			} else {
				fmt.Printf("Error: Unknown library '%s'\n", libName)
				return
			}
			continue
		}

		for customLib, goPkg := range activeLibRenames {
			line = strings.ReplaceAll(line, customLib+".", goPkg+".")
		}

		if strings.HasPrefix(line, "alias ") {
			parts := strings.Split(strings.TrimPrefix(line, "alias"), "=")
			if len(parts) == 2 {
				aliasName := strings.TrimSpace(parts[0])
				originalCmd := strings.TrimSpace(parts[1])
				aliases[aliasName] = originalCmd
			}
			continue
		}

		for aliasName, originalCmd := range aliases {
			if strings.HasPrefix(line, aliasName+" ") || line == aliasName {
				line = strings.Replace(line, aliasName, originalCmd, 1)
				break
			}
		}

		if strings.HasPrefix(line, "var {") && strings.Contains(line, "require") {
			openBrace := strings.Index(line, "{")
			closeBrace := strings.Index(line, "}")
			if openBrace != -1 && closeBrace != -1 {
				keysPart := line[openBrace+1 : closeBrace]
				rawKeys := strings.Split(keysPart, ",")
				var keys []string
				for _, k := range rawKeys {
					k = strings.TrimSpace(k)
					if k != "" {
						keys = append(keys, k)
					}
				}

				requirePart := line[closeBrace+1:]
				q1 := strings.Index(requirePart, "\"")
				q2 := strings.LastIndex(requirePart, "\"")
				jsonFile := ""
				if q1 != -1 && q2 != -1 && q2 > q1 {
					jsonFile = requirePart[q1+1 : q2]
				}

				for _, k := range keys {
					translatedLines = append(translatedLines, fmt.Sprintf("var %s any", k))
				}
				translatedLines = append(translatedLines, fmt.Sprintf("func() {\n\tf, err := os.Open(%q)\n\tif err == nil {\n\t\tdefer f.Close()\n\t\tvar m map[string]any\n\t\tjson.NewDecoder(f).Decode(&m)", jsonFile))
				for _, k := range keys {
					translatedLines = append(translatedLines, fmt.Sprintf("\t\tif val, ok := m[%q]; ok { %s = val }", k, k))
				}
				translatedLines = append(translatedLines, "\t}\n}()")
				continue
			}
		}

		if strings.HasPrefix(line, "if ") || strings.HasPrefix(line, "elif ") || strings.HasPrefix(line, "else ") {
			prefix := "if "
			if strings.HasPrefix(line, "elif ") {
				prefix = "elif "
			} else if strings.HasPrefix(line, "else ") {
				prefix = "else "
			}

			headerEnd := strings.Index(line, "{")
			if headerEnd == -1 {
				headerEnd = len(line)
			}
			
			conditionPart := strings.TrimSpace(line[len(prefix):headerEnd])
			conditionPart = strings.ReplaceAll(conditionPart, "ñ", "nil")
			conditionPart = strings.ReplaceAll(conditionPart, "True", "true")
			conditionPart = strings.ReplaceAll(conditionPart, "False", "false")

			var goIfLine string
			if prefix == "else " {
				if conditionPart != "" {
					if strings.HasPrefix(conditionPart, "(") && strings.HasSuffix(conditionPart, ")") {
						conditionPart = strings.TrimSpace(conditionPart[1 : len(conditionPart)-1])
					}
					goIfLine = fmt.Sprintf("else if %s {", conditionPart)
				} else {
					goIfLine = "else {"
				}
			} else if prefix == "elif " {
				if strings.HasPrefix(conditionPart, "(") && strings.HasSuffix(conditionPart, ")") {
					conditionPart = strings.TrimSpace(conditionPart[1 : len(conditionPart)-1])
				}
				goIfLine = fmt.Sprintf("else if %s {", conditionPart)
			} else {
				if strings.HasPrefix(conditionPart, "(") && strings.HasSuffix(conditionPart, ")") {
					conditionPart = strings.TrimSpace(conditionPart[1 : len(conditionPart)-1])
				}
				goIfLine = fmt.Sprintf("if %s {", conditionPart)
			}

			if (strings.HasPrefix(goIfLine, "else if ") || strings.HasPrefix(goIfLine, "else {")) && len(translatedLines) > 0 {
				lastIdx := len(translatedLines) - 1
				if strings.HasSuffix(translatedLines[lastIdx], "}") {
					translatedLines[lastIdx] = translatedLines[lastIdx] + " " + goIfLine
				} else {
					translatedLines = append(translatedLines, goIfLine)
				}
			} else {
				translatedLines = append(translatedLines, goIfLine)
			}

			for scanner.Scan() {
				innerLine := strings.TrimSpace(scanner.Text())
				if innerLine == "}" {
					translatedLines = append(translatedLines, "}")
					break
				}
				if innerLine == "" || strings.HasPrefix(innerLine, "#") {
					continue
				}
				for customLib, goPkg := range activeLibRenames {
					innerLine = strings.ReplaceAll(innerLine, customLib+".", goPkg+".")
				}
				translateInnerLine(innerLine, &translatedLines)
			}
			continue
		}

		if strings.HasPrefix(line, "struct ") {
			parts := strings.Fields(strings.TrimPrefix(line, "struct"))
			if len(parts) > 0 {
				structName := parts[0]
				translatedLines = append(translatedLines, fmt.Sprintf("type %s struct {", structName))
				fieldsPart := strings.Join(parts[1:], " ")
				rawFields := strings.Split(fieldsPart, ",")
				for _, f := range rawFields {
					f = strings.TrimSpace(f)
					if f != "" {
						if !strings.Contains(f, " ") {
							translatedLines = append(translatedLines, fmt.Sprintf("\t%s any", f))
						} else {
							translatedLines = append(translatedLines, fmt.Sprintf("\t%s", f))
						}
					}
				}
				translatedLines = append(translatedLines, "}")
			}
			continue
		}

		if strings.HasPrefix(line, "method ") {
			headerEnd := strings.Index(line, "{")
			if headerEnd == -1 {
				headerEnd = len(line)
			}
			header := strings.TrimSpace(line[7:headerEnd])
			parts := strings.Fields(header)
			if len(parts) >= 2 {
				structName := parts[0]
				methodSig := strings.Join(parts[1:], " ")
				
				openParen := strings.Index(methodSig, "(")
				closeParen := strings.LastIndex(methodSig, ")")
				var methodName string
				var paramsStr string
				if openParen != -1 && closeParen != -1 {
					methodName = strings.TrimSpace(methodSig[:openParen])
					paramsStr = strings.TrimSpace(methodSig[openParen+1 : closeParen])
				} else {
					methodName = strings.TrimSpace(methodSig)
				}

				var processedParams []string
				if paramsStr != "" {
					rawParams := strings.Split(paramsStr, ",")
					for _, p := range rawParams {
						p = strings.TrimSpace(p)
						if p != "" {
							if !strings.Contains(p, " ") {
								p = p + " any"
							}
							processedParams = append(processedParams, p)
						}
					}
				}
				goParams := strings.Join(processedParams, ", ")
				receiverName := strings.ToLower(structName[:1])
				translatedLines = append(translatedLines, fmt.Sprintf("func (%s *%s) %s(%s) {", receiverName, structName, methodName, goParams))

				for scanner.Scan() {
					innerLine := strings.TrimSpace(scanner.Text())
					if innerLine == "}" {
						translatedLines = append(translatedLines, "}")
						break
					}
					if innerLine == "" || strings.HasPrefix(innerLine, "#") {
						continue
					}
					for customLib, goPkg := range activeLibRenames {
						innerLine = strings.ReplaceAll(innerLine, customLib+".", goPkg+".")
					}
					translateInnerLine(innerLine, &translatedLines)
				}
			}
			continue
		}

		if strings.HasPrefix(line, "constructor ") {
			headerEnd := strings.Index(line, "{")
			if headerEnd == -1 {
				headerEnd = len(line)
			}
			header := strings.TrimSpace(line[11:headerEnd])
			
			parts := strings.Fields(header)
			if len(parts) >= 2 {
				structName := parts[0]
				funcPart := strings.Join(parts[1:], " ")
				
				openParen := strings.Index(funcPart, "(")
				closeParen := strings.LastIndex(funcPart, ")")
				
				var funcName string
				var paramsStr string
				if openParen != -1 && closeParen != -1 {
					funcName = strings.TrimSpace(funcPart[:openParen])
					paramsStr = strings.TrimSpace(funcPart[openParen+1 : closeParen])
				} else {
					fnParts := strings.Fields(funcPart)
					if len(fnParts) > 0 {
						funcName = fnParts[0]
						if len(fnParts) > 1 {
							paramsStr = strings.Join(fnParts[1:], " ")
						}
					}
				}

				var processedParams []string
				var fieldAssignments []string
				if paramsStr != "" {
					rawParams := strings.Split(paramsStr, ",")
					for _, p := range rawParams {
						p = strings.TrimSpace(p)
						if p != "" {
							p = strings.Trim(p, "()")
							if p == "" {
								continue
							}
							subParams := strings.Fields(p)
							paramName := subParams[0]
							fieldAssignments = append(fieldAssignments, fmt.Sprintf("%s: %s", paramName, paramName))
							
							if !strings.Contains(p, " ") {
								p = p + " any"
							}
							processedParams = append(processedParams, p)
						}
					}
				}
				goParams := strings.Join(processedParams, ", ")
				
				translatedLines = append(translatedLines, fmt.Sprintf("func %s(%s) *%s {", funcName, goParams, structName))
				translatedLines = append(translatedLines, fmt.Sprintf("\treturn &%s{%s}", structName, strings.Join(fieldAssignments, ", ")))
				translatedLines = append(translatedLines, "}")
			}
			continue
		}

		if strings.HasPrefix(line, "func ") || strings.HasPrefix(line, "fn ") {
			isStrict := strings.HasPrefix(line, "fn ")
			prefixLen := 4
			if isStrict {
				prefixLen = 3
			}

			headerEnd := strings.Index(line, "{")
			if headerEnd == -1 {
				continue
			}
			
			header := strings.TrimSpace(line[prefixLen:headerEnd])
			parts := strings.Split(header, ":")
			funcSignature := strings.TrimSpace(parts[0])
			
			outputType := ""
			if len(parts) > 1 {
				outputType = strings.TrimSpace(parts[1])
				outputType = strings.TrimPrefix(outputType, "{")
				outputType = strings.TrimSuffix(outputType, "}")
				outputType = strings.TrimPrefix(outputType, "(")
				outputType = strings.TrimSuffix(outputType, ")")
				outputType = strings.TrimSpace(outputType)
			}

			openParen := strings.Index(funcSignature, "(")
			closeParen := strings.LastIndex(funcSignature, ")")
			if openParen == -1 || closeParen == -1 {
				continue
			}

			funcName := strings.TrimSpace(funcSignature[:openParen])
			paramsStr := strings.TrimSpace(funcSignature[openParen+1 : closeParen])

			var processedParams []string
			if paramsStr != "" {
				rawParams := strings.Split(paramsStr, ",")
				for _, p := range rawParams {
					p = strings.TrimSpace(p)
					if p == "" {
						continue
					}
					if !strings.Contains(p, " ") && !isStrict {
						p = p + " any"
					}
					processedParams = append(processedParams, p)
				}
			}

			goParams := strings.Join(processedParams, ", ")
			goFuncSignature := fmt.Sprintf("func %s(%s)", funcName, goParams)
			if outputType != "" {
				goFuncSignature += " " + outputType
			}
			goFuncSignature += " {"

			translatedLines = append(translatedLines, goFuncSignature)

			for scanner.Scan() {
				innerLine := strings.TrimSpace(scanner.Text())
				if innerLine == "}" {
					translatedLines = append(translatedLines, "}")
					break
				}

				if innerLine == "" || strings.HasPrefix(innerLine, "#") {
					continue
				}

				for customLib, goPkg := range activeLibRenames {
					innerLine = strings.ReplaceAll(innerLine, customLib+".", goPkg+".")
				}

				translateInnerLine(innerLine, &translatedLines)
			}
			continue
		}

		if strings.HasPrefix(line, "for ") {
			headerEnd := strings.Index(line, "{")
			if headerEnd == -1 {
				continue
			}
			header := strings.TrimSpace(line[4:headerEnd])
			parts := strings.Split(header, "|")
			if len(parts) != 3 {
				continue
			}
			init := strings.TrimSpace(parts[0])
			cond := strings.TrimSpace(parts[1])
			post := strings.TrimSpace(parts[2])

			if strings.Contains(init, "=") && !strings.Contains(init, ":=") {
				initParts := strings.SplitN(init, "=", 2)
				init = strings.TrimSpace(initParts[0]) + " := " + strings.TrimSpace(initParts[1])
			}

			cond = strings.ReplaceAll(cond, "ñ", "nil")
			cond = strings.ReplaceAll(cond, "True", "true")
			cond = strings.ReplaceAll(cond, "False", "false")
			post = strings.ReplaceAll(post, "ñ", "nil")
			translatedLines = append(translatedLines, fmt.Sprintf("for %s; %s; %s {", init, cond, post))

			for scanner.Scan() {
				innerLine := strings.TrimSpace(scanner.Text())
				if innerLine == "}" {
					translatedLines = append(translatedLines, "}")
					break
				}
				if innerLine == "" || strings.HasPrefix(innerLine, "#") {
					continue
				}
				for customLib, goPkg := range activeLibRenames {
					innerLine = strings.ReplaceAll(innerLine, customLib+".", goPkg+".")
				}
				translateInnerLine(innerLine, &translatedLines)
			}
			continue
		}

		if strings.HasPrefix(line, "while ") {
			headerEnd := strings.Index(line, "{")
			if headerEnd == -1 {
				continue
			}
			cond := strings.TrimSpace(line[5:headerEnd])
			if strings.HasPrefix(cond, "(") && strings.HasSuffix(cond, ")") {
				cond = strings.TrimSpace(cond[1 : len(cond)-1])
			}

			cond = strings.ReplaceAll(cond, "ñ", "nil")
			cond = strings.ReplaceAll(cond, "True", "true")
			cond = strings.ReplaceAll(cond, "False", "false")
			translatedLines = append(translatedLines, fmt.Sprintf("for %s {", cond))

			for scanner.Scan() {
				innerLine := strings.TrimSpace(scanner.Text())
				if innerLine == "}" {
					translatedLines = append(translatedLines, "}")
					break
				}
				if innerLine == "" || strings.HasPrefix(innerLine, "#") {
					continue
				}
				for customLib, goPkg := range activeLibRenames {
					innerLine = strings.ReplaceAll(innerLine, customLib+".", goPkg+".")
				}
				translateInnerLine(innerLine, &translatedLines)
			}
			continue
		}

		if strings.HasPrefix(line, "print ") {
			target := strings.TrimSpace(strings.TrimPrefix(line, "print"))
			target = strings.ReplaceAll(target, "ñ", "nil")
			target = strings.ReplaceAll(target, "True", "true")
			target = strings.ReplaceAll(target, "False", "false")
			translatedLines = append(translatedLines, fmt.Sprintf("fmt.Println(%s)", target))

		} else if strings.HasPrefix(line, "return ") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "return"))
			val = strings.ReplaceAll(val, "ñ", "nil")
			val = strings.ReplaceAll(val, "True", "true")
			val = strings.ReplaceAll(val, "False", "false")
			if strings.HasPrefix(val, "(") && strings.HasSuffix(val, ")") {
				val = strings.TrimSpace(val[1 : len(val)-1])
			}
			translatedLines = append(translatedLines, fmt.Sprintf("return %s", val))

		} else if strings.HasPrefix(line, "input ") {
			promptText := strings.TrimSpace(strings.TrimPrefix(line, "input"))
			promptText = strings.ReplaceAll(promptText, "ñ", "nil")
			translatedLines = append(translatedLines, fmt.Sprintf("input(%s)", promptText))
			
		} else if strings.HasPrefix(line, "lower ") {
			param := strings.TrimSpace(strings.TrimPrefix(line, "lower"))
			param = strings.ReplaceAll(param, "ñ", "nil")
			translatedLines = append(translatedLines, fmt.Sprintf("lower(%s)", param))

		} else if strings.HasPrefix(line, "upper ") {
			param := strings.TrimSpace(strings.TrimPrefix(line, "upper"))
			param = strings.ReplaceAll(param, "ñ", "nil")
			translatedLines = append(translatedLines, fmt.Sprintf("upper(%s)", param))

		} else if strings.HasPrefix(line, "ivs ") {
			param := strings.TrimSpace(strings.TrimPrefix(line, "ivs"))
			param = strings.ReplaceAll(param, "ñ", "nil")
			translatedLines = append(translatedLines, fmt.Sprintf("ivs(%s)", param))

		} else if strings.HasPrefix(line, "len ") {
			param := strings.TrimSpace(strings.TrimPrefix(line, "len"))
			param = strings.ReplaceAll(param, "ñ", "nil")
			translatedLines = append(translatedLines, fmt.Sprintf("len(%s)", param))

		} else if strings.HasPrefix(line, "string ") {
			param := strings.TrimSpace(strings.TrimPrefix(line, "string"))
			param = strings.ReplaceAll(param, "ñ", "nil")
			translatedLines = append(translatedLines, fmt.Sprintf("toString(%s)", param))

		} else if strings.HasPrefix(line, "int ") {
			param := strings.TrimSpace(strings.TrimPrefix(line, "int"))
			param = strings.ReplaceAll(param, "ñ", "nil")
			translatedLines = append(translatedLines, fmt.Sprintf("toInt(%s)", param))

		} else if strings.HasPrefix(line, "float ") {
			param := strings.TrimSpace(strings.TrimPrefix(line, "float"))
			param = strings.ReplaceAll(param, "ñ", "nil")
			translatedLines = append(translatedLines, fmt.Sprintf("toFloat(%s)", param))

		} else if strings.HasPrefix(line, "bool ") {
			param := strings.TrimSpace(strings.TrimPrefix(line, "bool"))
			param = strings.ReplaceAll(param, "ñ", "nil")
			param = strings.ReplaceAll(param, "True", "true")
			param = strings.ReplaceAll(param, "False", "false")
			translatedLines = append(translatedLines, fmt.Sprintf("toBool(%s)", param))

		} else if strings.HasPrefix(line, "round ") {
			param := strings.TrimSpace(strings.TrimPrefix(line, "round"))
			param = strings.ReplaceAll(param, "ñ", "nil")
			if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
				param = strings.TrimSpace(param[1 : len(param)-1])
			}
			translatedLines = append(translatedLines, fmt.Sprintf("round(%s)", param))

		} else if strings.HasPrefix(line, "choice ") {
			param := strings.TrimSpace(strings.TrimPrefix(line, "choice"))
			param = strings.ReplaceAll(param, "ñ", "nil")
			if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
				param = strings.TrimSpace(param[1 : len(param)-1])
			}
			translatedLines = append(translatedLines, fmt.Sprintf("choice(%s)", param))

		} else if strings.HasPrefix(line, "randint ") {
			param := strings.TrimSpace(strings.TrimPrefix(line, "randint"))
			param = strings.ReplaceAll(param, "ñ", "nil")
			if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
				param = strings.TrimSpace(param[1 : len(param)-1])
			}
			if !strings.Contains(param, ",") && strings.Contains(param, " ") {
				fields := strings.Fields(param)
				if len(fields) == 2 {
					param = strings.Join(fields, ", ")
				}
			}
			translatedLines = append(translatedLines, fmt.Sprintf("randint(%s)", param))

		} else if strings.HasPrefix(line, "randfloat ") {
			param := strings.TrimSpace(strings.TrimPrefix(line, "randfloat"))
			param = strings.ReplaceAll(param, "ñ", "nil")
			if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
				param = strings.TrimSpace(param[1 : len(param)-1])
			}
			if !strings.Contains(param, ",") && strings.Contains(param, " ") {
				fields := strings.Fields(param)
				if len(fields) == 2 {
					param = strings.Join(fields, ", ")
				}
			}
			translatedLines = append(translatedLines, fmt.Sprintf("randfloat(%s)", param))

		} else if strings.HasPrefix(line, "var ") || strings.HasPrefix(line, "const ") {
			isConst := strings.HasPrefix(line, "const ")
			prefixLen := 4
			if isConst {
				prefixLen = 6
			}

			parts := strings.SplitN(strings.TrimSpace(line[prefixLen:]), " ", 3)
			if len(parts) < 3 {
				continue
			}

			varName := parts[0]
			rawVal := strings.TrimSpace(strings.TrimPrefix(parts[2], "="))
			
			var assignment string
			if rawVal == "ñ" {
				assignment = fmt.Sprintf("var %s any = nil", varName)
			} else if rawVal == "True" || rawVal == "true" {
				assignment = fmt.Sprintf("%s := true", varName)
			} else if rawVal == "False" || rawVal == "false" {
				assignment = fmt.Sprintf("%s := false", varName)
			} else if strings.HasPrefix(rawVal, "input ") {
				promptText := strings.TrimSpace(strings.TrimPrefix(rawVal, "input"))
				promptText = strings.ReplaceAll(promptText, "ñ", "nil")
				assignment = fmt.Sprintf("%s := input(%s)", varName, promptText)
			} else if strings.HasPrefix(rawVal, "lower ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "lower"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				assignment = fmt.Sprintf("%s := lower(%s)", varName, param)
			} else if strings.HasPrefix(rawVal, "upper ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "upper"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				assignment = fmt.Sprintf("%s := upper(%s)", varName, param)
			} else if strings.HasPrefix(rawVal, "ivs ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "ivs"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				assignment = fmt.Sprintf("%s := ivs(%s)", varName, param)
			} else if strings.HasPrefix(rawVal, "len ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "len"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				assignment = fmt.Sprintf("%s := len(%s)", varName, param)
			} else if strings.HasPrefix(rawVal, "string ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "string"))
				param = strings.ReplaceAll(rawVal, "ñ", "nil")
				assignment = fmt.Sprintf("%s := toString(%s)", varName, param)
			} else if strings.HasPrefix(rawVal, "int ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "int"))
				param = strings.ReplaceAll(rawVal, "ñ", "nil")
				assignment = fmt.Sprintf("%s := toInt(%s)", varName, param)
			} else if strings.HasPrefix(rawVal, "float ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "float"))
				param = strings.ReplaceAll(rawVal, "ñ", "nil")
				assignment = fmt.Sprintf("%s := toFloat(%s)", varName, param)
			} else if strings.HasPrefix(rawVal, "bool ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "bool"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				param = strings.ReplaceAll(param, "True", "true")
				param = strings.ReplaceAll(param, "False", "false")
				assignment = fmt.Sprintf("%s := toBool(%s)", varName, param)
			} else if strings.HasPrefix(rawVal, "round ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "round"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
					param = strings.TrimSpace(param[1 : len(param)-1])
				}
				assignment = fmt.Sprintf("%s := round(%s)", varName, param)
			} else if strings.HasPrefix(rawVal, "choice ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "choice"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
					param = strings.TrimSpace(param[1 : len(param)-1])
				}
				assignment = fmt.Sprintf("%s := choice(%s)", varName, param)
			} else if strings.HasPrefix(rawVal, "randint ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "randint"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
					param = strings.TrimSpace(param[1 : len(param)-1])
				}
				if !strings.Contains(param, ",") && strings.Contains(param, " ") {
					fields := strings.Fields(param)
					if len(fields) == 2 {
						param = strings.Join(fields, ", ")
					}
				}
				assignment = fmt.Sprintf("%s := randint(%s)", varName, param)
			} else if strings.HasPrefix(rawVal, "randfloat ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "randfloat"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
					param = strings.TrimSpace(param[1 : len(param)-1])
				}
				if !strings.Contains(param, ",") && strings.Contains(param, " ") {
					fields := strings.Fields(param)
					if len(fields) == 2 {
						param = strings.Join(fields, ", ")
					}
				}
				assignment = fmt.Sprintf("%s := randfloat(%s)", varName, param)
			} else {
				rawVal = strings.ReplaceAll(rawVal, "ñ", "nil")
				rawVal = strings.ReplaceAll(rawVal, "True", "true")
				rawVal = strings.ReplaceAll(rawVal, "False", "false")
				if strings.HasPrefix(rawVal, "[") && strings.HasSuffix(rawVal, "]") {
					rawVal = "[]any{" + rawVal[1:len(rawVal)-1] + "}"
				}
				assignment = fmt.Sprintf("%s := %s", varName, rawVal)
			}
			translatedLines = append(translatedLines, assignment)
		} else {
			if strings.Contains(line, "= input ") {
				parts := strings.SplitN(line, "= input ", 2)
				lhs := strings.TrimSpace(parts[0])
				promptText := strings.TrimSpace(parts[1])
				line = fmt.Sprintf("%s = input(%s)", lhs, promptText)
			} else if strings.Contains(line, "= lower ") {
				parts := strings.SplitN(line, "= lower ", 2)
				lhs := strings.TrimSpace(parts[0])
				param := strings.TrimSpace(parts[1])
				line = fmt.Sprintf("%s = lower(%s)", lhs, param)
			} else if strings.Contains(line, "= upper ") {
				parts := strings.SplitN(line, "= upper ", 2)
				lhs := strings.TrimSpace(parts[0])
				param := strings.TrimSpace(parts[1])
				line = fmt.Sprintf("%s = upper(%s)", lhs, param)
			} else if strings.Contains(line, "= ivs ") {
				parts := strings.SplitN(line, "= ivs ", 2)
				lhs := strings.TrimSpace(parts[0])
				param := strings.TrimSpace(parts[1])
				line = fmt.Sprintf("%s = ivs(%s)", lhs, param)
			} else if strings.Contains(line, "= len ") {
				parts := strings.SplitN(line, "= len ", 2)
				lhs := strings.TrimSpace(parts[0])
				param := strings.TrimSpace(parts[1])
				line = fmt.Sprintf("%s = len(%s)", lhs, param)
			} else if strings.Contains(line, "= string ") {
				parts := strings.SplitN(line, "= string ", 2)
				lhs := strings.TrimSpace(parts[0])
				param := strings.TrimSpace(parts[1])
				line = fmt.Sprintf("%s = toString(%s)", lhs, param)
			} else if strings.Contains(line, "= int ") {
				parts := strings.SplitN(line, "= int ", 2)
				lhs := strings.TrimSpace(parts[0])
				param := strings.TrimSpace(parts[1])
				line = fmt.Sprintf("%s = toInt(%s)", lhs, param)
			} else if strings.Contains(line, "= float ") {
				parts := strings.SplitN(line, "= float ", 2)
				lhs := strings.TrimSpace(parts[0])
				param := strings.TrimSpace(parts[1])
				line = fmt.Sprintf("%s = toFloat(%s)", lhs, param)
			} else if strings.Contains(line, "= bool ") {
				parts := strings.SplitN(line, "= bool ", 2)
				lhs := strings.TrimSpace(parts[0])
				param := strings.TrimSpace(parts[1])
				param = strings.ReplaceAll(param, "True", "true")
				param = strings.ReplaceAll(param, "False", "false")
				line = fmt.Sprintf("%s = toBool(%s)", lhs, param)
			} else if strings.Contains(line, "= round ") {
				parts := strings.SplitN(line, "= round ", 2)
				lhs := strings.TrimSpace(parts[0])
				param := strings.TrimSpace(parts[1])
				if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
					param = strings.TrimSpace(param[1 : len(param)-1])
				}
				line = fmt.Sprintf("%s = round(%s)", lhs, param)
			} else if strings.Contains(line, "= choice ") {
				parts := strings.SplitN(line, "= choice ", 2)
				lhs := strings.TrimSpace(parts[0])
				param := strings.TrimSpace(parts[1])
				if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
					param = strings.TrimSpace(param[1 : len(param)-1])
				}
				line = fmt.Sprintf("%s = choice(%s)", lhs, param)
			} else if strings.Contains(line, "= randint ") {
				parts := strings.SplitN(line, "= randint ", 2)
				lhs := strings.TrimSpace(parts[0])
				param := strings.TrimSpace(parts[1])
				if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
					param = strings.TrimSpace(param[1 : len(param)-1])
				}
				if !strings.Contains(param, ",") && strings.Contains(param, " ") {
					fields := strings.Fields(param)
					if len(fields) == 2 {
						param = strings.Join(fields, ", ")
					}
				}
				line = fmt.Sprintf("%s = randint(%s)", lhs, param)
			} else if strings.Contains(line, "= randfloat ") {
				parts := strings.SplitN(line, "= randfloat ", 2)
				lhs := strings.TrimSpace(parts[0])
				param := strings.TrimSpace(parts[1])
				if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
					param = strings.TrimSpace(param[1 : len(param)-1])
				}
				if !strings.Contains(param, ",") && strings.Contains(param, " ") {
					fields := strings.Fields(param)
					if len(fields) == 2 {
						param = strings.Join(fields, ", ")
					}
				}
				line = fmt.Sprintf("%s = randfloat(%s)", lhs, param)
			}
			
			line = strings.ReplaceAll(line, "ñ", "nil")
			line = strings.ReplaceAll(line, "True", "true")
			line = strings.ReplaceAll(line, "False", "false")
			if strings.Contains(line, "=") {
				eqParts := strings.SplitN(line, "=", 2)
				if len(eqParts) == 2 {
					lhs := strings.TrimSpace(eqParts[0])
					rhs := strings.TrimSpace(eqParts[1])
					if strings.HasPrefix(rhs, "[") && strings.HasSuffix(rhs, "]") {
						rhs = "[]any{" + rhs[1:len(rhs)-1] + "}"
					}
					line = fmt.Sprintf("%s = %s", lhs, rhs)
				}
				translatedLines = append(translatedLines, line)
			} else if strings.Contains(line, "(") && strings.Contains(line, ")") {
				translatedLines = append(translatedLines, line)
			} else {
				fmt.Printf("Unknown Command -> '%s'\n", line)
			}
		}
	}

	generateAndRunGoFile(translatedImports, translatedLines)
}

func translateInnerLine(line string, translatedLines *[]string) {
	if strings.HasPrefix(line, "if ") || strings.HasPrefix(line, "elif ") || strings.HasPrefix(line, "else ") {
		prefix := "if "
		if strings.HasPrefix(line, "elif ") {
			prefix = "elif "
		} else if strings.HasPrefix(line, "else ") {
			prefix = "else "
		}

		headerEnd := strings.Index(line, "{")
		if headerEnd == -1 {
			headerEnd = len(line)
		}
		
		conditionPart := strings.TrimSpace(line[len(prefix):headerEnd])
		conditionPart = strings.ReplaceAll(conditionPart, "ñ", "nil")
		conditionPart = strings.ReplaceAll(conditionPart, "True", "true")
		conditionPart = strings.ReplaceAll(conditionPart, "False", "false")

		var goIfLine string
		if prefix == "else " {
			if conditionPart != "" {
				if strings.HasPrefix(conditionPart, "(") && strings.HasSuffix(conditionPart, ")") {
					conditionPart = strings.TrimSpace(conditionPart[1 : len(conditionPart)-1])
				}
				goIfLine = fmt.Sprintf("\telse if %s {", conditionPart)
			} else {
				goIfLine = "\telse {"
			}
		} else if prefix == "elif " {
			if strings.HasPrefix(conditionPart, "(") && strings.HasSuffix(conditionPart, ")") {
				conditionPart = strings.TrimSpace(conditionPart[1 : len(conditionPart)-1])
			}
			goIfLine = fmt.Sprintf("\telse if %s {", conditionPart)
		} else {
			if strings.HasPrefix(conditionPart, "(") && strings.HasSuffix(conditionPart, ")") {
				conditionPart = strings.TrimSpace(conditionPart[1 : len(conditionPart)-1])
			}
			goIfLine = fmt.Sprintf("\tif %s {", conditionPart)
		}

		if (strings.HasPrefix(strings.TrimSpace(goIfLine), "else if ") || strings.HasPrefix(strings.TrimSpace(goIfLine), "else {")) && len(*translatedLines) > 0 {
			lastIdx := len(*translatedLines) - 1
			if strings.HasSuffix((*translatedLines)[lastIdx], "}") {
				(*translatedLines)[lastIdx] = (*translatedLines)[lastIdx] + " " + strings.TrimSpace(goIfLine)
			} else {
				*translatedLines = append(*translatedLines, goIfLine)
			}
		} else {
			*translatedLines = append(*translatedLines, goIfLine)
		}
		return
	}

	if strings.HasPrefix(line, "var {") && strings.Contains(line, "require") {
		openBrace := strings.Index(line, "{")
		closeBrace := strings.Index(line, "}")
		if openBrace != -1 && closeBrace != -1 {
			keysPart := line[openBrace+1 : closeBrace]
			rawKeys := strings.Split(keysPart, ",")
			var keys []string
			for _, k := range rawKeys {
				k = strings.TrimSpace(k)
				if k != "" {
					keys = append(keys, k)
				}
			}

			requirePart := line[closeBrace+1:]
			q1 := strings.Index(requirePart, "\"")
			q2 := strings.LastIndex(requirePart, "\"")
			jsonFile := ""
			if q1 != -1 && q2 != -1 && q2 > q1 {
				jsonFile = requirePart[q1+1 : q2]
			}

			for _, k := range keys {
				*translatedLines = append(*translatedLines, fmt.Sprintf("\tvar %s any", k))
			}
			*translatedLines = append(*translatedLines, fmt.Sprintf("\tfunc() {\n\t\tf, err := os.Open(%q)\n\t\tif err == nil {\n\t\t\tdefer f.Close()\n\t\t\tvar m map[string]any\n\t\t\tjson.NewDecoder(f).Decode(&m)", jsonFile))
			for _, k := range keys {
				*translatedLines = append(*translatedLines, fmt.Sprintf("\t\t\tif val, ok := m[%q]; ok { %s = val }", k, k))
			}
			*translatedLines = append(*translatedLines, "\t\t}\n\t}()")
			return
		}
	}

	if strings.HasPrefix(line, "print ") {
		target := strings.TrimSpace(strings.TrimPrefix(line, "print"))
		target = strings.ReplaceAll(target, "ñ", "nil")
		target = strings.ReplaceAll(target, "True", "true")
		target = strings.ReplaceAll(target, "False", "false")
		*translatedLines = append(*translatedLines, fmt.Sprintf("\tfmt.Println(%s)", target))
	} else if strings.HasPrefix(line, "return ") {
		val := strings.TrimSpace(strings.TrimPrefix(line, "return"))
		val = strings.ReplaceAll(val, "ñ", "nil")
		val = strings.ReplaceAll(val, "True", "true")
		val = strings.ReplaceAll(val, "False", "false")
		if strings.HasPrefix(val, "(") && strings.HasSuffix(val, ")") {
			val = strings.TrimSpace(val[1 : len(val)-1])
		}
		*translatedLines = append(*translatedLines, fmt.Sprintf("\treturn %s", val))
	} else if strings.HasPrefix(line, "input ") {
		promptText := strings.TrimSpace(strings.TrimPrefix(line, "input"))
		promptText = strings.ReplaceAll(promptText, "ñ", "nil")
		*translatedLines = append(*translatedLines, fmt.Sprintf("\tinput(%s)", promptText))
	} else if strings.HasPrefix(line, "lower ") {
		param := strings.TrimSpace(strings.TrimPrefix(line, "lower"))
		param = strings.ReplaceAll(param, "ñ", "nil")
		*translatedLines = append(*translatedLines, fmt.Sprintf("\tlower(%s)", param))
	} else if strings.HasPrefix(line, "upper ") {
		param := strings.TrimSpace(strings.TrimPrefix(line, "upper"))
		param = strings.ReplaceAll(param, "ñ", "nil")
		*translatedLines = append(*translatedLines, fmt.Sprintf("\tupper(%s)", param))
	} else if strings.HasPrefix(line, "ivs ") {
		param := strings.TrimSpace(strings.TrimPrefix(line, "ivs"))
		param = strings.ReplaceAll(param, "ñ", "nil")
		*translatedLines = append(*translatedLines, fmt.Sprintf("\tivs(%s)", param))
	} else if strings.HasPrefix(line, "len ") {
		param := strings.TrimSpace(strings.TrimPrefix(line, "len"))
		param = strings.ReplaceAll(param, "ñ", "nil")
		*translatedLines = append(*translatedLines, fmt.Sprintf("\tlen(%s)", param))
	} else if strings.HasPrefix(line, "string ") {
		param := strings.TrimSpace(strings.TrimPrefix(line, "string"))
		param = strings.ReplaceAll(param, "ñ", "nil")
		*translatedLines = append(*translatedLines, fmt.Sprintf("\ttoString(%s)", param))
	} else if strings.HasPrefix(line, "int ") {
		param := strings.TrimSpace(strings.TrimPrefix(line, "int"))
		param = strings.ReplaceAll(param, "ñ", "nil")
		*translatedLines = append(*translatedLines, fmt.Sprintf("\ttoInt(%s)", param))
	} else if strings.HasPrefix(line, "float ") {
		param := strings.TrimSpace(strings.TrimPrefix(line, "float"))
		param = strings.ReplaceAll(param, "ñ", "nil")
		*translatedLines = append(*translatedLines, fmt.Sprintf("\ttoFloat(%s)", param))
	} else if strings.HasPrefix(line, "bool ") {
		param := strings.TrimSpace(strings.TrimPrefix(line, "bool"))
		param = strings.ReplaceAll(param, "ñ", "nil")
		param = strings.ReplaceAll(param, "True", "true")
		param = strings.ReplaceAll(param, "False", "false")
		*translatedLines = append(*translatedLines, fmt.Sprintf("\ttoBool(%s)", param))
	} else if strings.HasPrefix(line, "round ") {
		param := strings.TrimSpace(strings.TrimPrefix(line, "round"))
		param = strings.ReplaceAll(param, "ñ", "nil")
		if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
			param = strings.TrimSpace(param[1 : len(param)-1])
		}
		*translatedLines = append(*translatedLines, fmt.Sprintf("\tround(%s)", param))
	} else if strings.HasPrefix(line, "choice ") {
		param := strings.TrimSpace(strings.TrimPrefix(line, "choice"))
		param = strings.ReplaceAll(param, "ñ", "nil")
		if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
			param = strings.TrimSpace(param[1 : len(param)-1])
		}
		*translatedLines = append(*translatedLines, fmt.Sprintf("\tchoice(%s)", param))
	} else if strings.HasPrefix(line, "randint ") {
		param := strings.TrimSpace(strings.TrimPrefix(line, "randint"))
		param = strings.ReplaceAll(param, "ñ", "nil")
		if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
			param = strings.TrimSpace(param[1 : len(param)-1])
		}
		if !strings.Contains(param, ",") && strings.Contains(param, " ") {
			fields := strings.Fields(param)
			if len(fields) == 2 {
				param = strings.Join(fields, ", ")
			}
		}
		*translatedLines = append(*translatedLines, fmt.Sprintf("\trandint(%s)", param))
	} else if strings.HasPrefix(line, "randfloat ") {
		param := strings.TrimSpace(strings.TrimPrefix(line, "randfloat"))
		param = strings.ReplaceAll(param, "ñ", "nil")
		if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
			param = strings.TrimSpace(param[1 : len(param)-1])
		}
		if !strings.Contains(param, ",") && strings.Contains(param, " ") {
			fields := strings.Fields(param)
			if len(fields) == 2 {
				param = strings.Join(fields, ", ")
			}
		}
		*translatedLines = append(*translatedLines, fmt.Sprintf("\trandfloat(%s)", param))
	} else if strings.HasPrefix(line, "var ") || strings.HasPrefix(line, "const ") {
		isConst := strings.HasPrefix(line, "const ")
		prefixLen := 4
		if isConst {
			prefixLen = 6
		}
		parts := strings.SplitN(strings.TrimSpace(line[prefixLen:]), " ", 3)
		if len(parts) >= 3 {
			varName := parts[0]
			rawVal := strings.TrimSpace(strings.TrimPrefix(parts[2], "="))
			if rawVal == "ñ" {
				*translatedLines = append(*translatedLines, fmt.Sprintf("\tvar %s any = nil", varName))
			} else if rawVal == "True" || rawVal == "true" {
				*translatedLines = append(*translatedLines, fmt.Sprintf("\t%s := true", varName))
			} else if rawVal == "False" || rawVal == "false" {
				*translatedLines = append(*translatedLines, fmt.Sprintf("\t%s := false", varName))
			} else if strings.HasPrefix(rawVal, "input ") {
				promptText := strings.TrimSpace(strings.TrimPrefix(rawVal, "input"))
				promptText = strings.ReplaceAll(promptText, "ñ", "nil")
				*translatedLines = append(*translatedLines, fmt.Sprintf("\t%s := input(%s)", varName, promptText))
			} else if strings.HasPrefix(rawVal, "lower ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "lower"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				*translatedLines = append(*translatedLines, fmt.Sprintf("\t%s := lower(%s)", varName, param))
			} else if strings.HasPrefix(rawVal, "upper ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "upper"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				*translatedLines = append(*translatedLines, fmt.Sprintf("\t%s := upper(%s)", varName, param))
			} else if strings.HasPrefix(rawVal, "ivs ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "ivs"))
				param = strings.ReplaceAll(rawVal, "ñ", "nil")
				*translatedLines = append(*translatedLines, fmt.Sprintf("\t%s := ivs(%s)", varName, param))
			} else if strings.HasPrefix(rawVal, "len ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "len"))
				param = strings.ReplaceAll(rawVal, "ñ", "nil")
				*translatedLines = append(*translatedLines, fmt.Sprintf("\t%s := len(%s)", varName, param))
			} else if strings.HasPrefix(rawVal, "string ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "string"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				*translatedLines = append(*translatedLines, fmt.Sprintf("\t%s := toString(%s)", varName, param))
			} else if strings.HasPrefix(rawVal, "int ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "int"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				*translatedLines = append(*translatedLines, fmt.Sprintf("\t%s := toInt(%s)", varName, param))
			} else if strings.HasPrefix(rawVal, "float ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "float"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				*translatedLines = append(*translatedLines, fmt.Sprintf("\t%s := toFloat(%s)", varName, param))
			} else if strings.HasPrefix(rawVal, "bool ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "bool"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				param = strings.ReplaceAll(param, "True", "true")
				param = strings.ReplaceAll(param, "False", "false")
				*translatedLines = append(*translatedLines, fmt.Sprintf("\t%s := toBool(%s)", varName, param))
			} else if strings.HasPrefix(rawVal, "round ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "round"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
					param = strings.TrimSpace(param[1 : len(param)-1])
				}
				*translatedLines = append(*translatedLines, fmt.Sprintf("\t%s := round(%s)", varName, param))
			} else if strings.HasPrefix(rawVal, "choice ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "choice"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
					param = strings.TrimSpace(param[1 : len(param)-1])
				}
				*translatedLines = append(*translatedLines, fmt.Sprintf("\t%s := choice(%s)", varName, param))
			} else if strings.HasPrefix(rawVal, "randint ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "randint"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
					param = strings.TrimSpace(param[1 : len(param)-1])
				}
				if !strings.Contains(param, ",") && strings.Contains(param, " ") {
					fields := strings.Fields(param)
					if len(fields) == 2 {
						param = strings.Join(fields, ", ")
					}
				}
				*translatedLines = append(*translatedLines, fmt.Sprintf("\t%s := randint(%s)", varName, param))
			} else if strings.HasPrefix(rawVal, "randfloat ") {
				param := strings.TrimSpace(strings.TrimPrefix(rawVal, "randfloat"))
				param = strings.ReplaceAll(param, "ñ", "nil")
				if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
					param = strings.TrimSpace(param[1 : len(param)-1])
				}
				if !strings.Contains(param, ",") && strings.Contains(param, " ") {
					fields := strings.Fields(param)
					if len(fields) == 2 {
						param = strings.Join(fields, ", ")
					}
				}
				*translatedLines = append(*translatedLines, fmt.Sprintf("\t%s := randfloat(%s)", varName, param))
			} else {
				rawVal = strings.ReplaceAll(rawVal, "ñ", "nil")
				rawVal = strings.ReplaceAll(rawVal, "True", "true")
				rawVal = strings.ReplaceAll(rawVal, "False", "false")
				if strings.HasPrefix(rawVal, "[") && strings.HasSuffix(rawVal, "]") {
					rawVal = "[]any{" + rawVal[1:len(rawVal)-1] + "}"
				}
				*translatedLines = append(*translatedLines, fmt.Sprintf("\t%s := %s", varName, rawVal))
			}
		}
	} else {
		if strings.Contains(line, "= round ") {
			parts := strings.SplitN(line, "= round ", 2)
			lhs := strings.TrimSpace(parts[0])
			param := strings.TrimSpace(parts[1])
			if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
				param = strings.TrimSpace(param[1 : len(param)-1])
			}
			line = fmt.Sprintf("%s = round(%s)", lhs, param)
		} else if strings.Contains(line, "= choice ") {
			parts := strings.SplitN(line, "= choice ", 2)
			lhs := strings.TrimSpace(parts[0])
			param := strings.TrimSpace(parts[1])
			if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
				param = strings.TrimSpace(param[1 : len(param)-1])
			}
			line = fmt.Sprintf("%s = choice(%s)", lhs, param)
		} else if strings.Contains(line, "= randint ") {
			parts := strings.SplitN(line, "= randint ", 2)
			lhs := strings.TrimSpace(parts[0])
			param := strings.TrimSpace(parts[1])
			if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
				param = strings.TrimSpace(param[1 : len(param)-1])
			}
			if !strings.Contains(param, ",") && strings.Contains(param, " ") {
				fields := strings.Fields(param)
				if len(fields) == 2 {
					param = strings.Join(fields, ", ")
				}
			}
			line = fmt.Sprintf("%s = randint(%s)", lhs, param)
		} else if strings.Contains(line, "= randfloat ") {
			parts := strings.SplitN(line, "= randfloat ", 2)
			lhs := strings.TrimSpace(parts[0])
			param := strings.TrimSpace(parts[1])
			if strings.HasPrefix(param, "(") && strings.HasSuffix(param, ")") {
				param = strings.TrimSpace(param[1 : len(param)-1])
			}
			if !strings.Contains(param, ",") && strings.Contains(param, " ") {
				fields := strings.Fields(param)
				if len(fields) == 2 {
					param = strings.Join(fields, ", ")
				}
			}
			line = fmt.Sprintf("%s = randfloat(%s)", lhs, param)
		} else if strings.Contains(line, "= bool ") {
			parts := strings.SplitN(line, "= bool ", 2)
			lhs := strings.TrimSpace(parts[0])
			param := strings.TrimSpace(parts[1])
			param = strings.ReplaceAll(param, "True", "true")
			param = strings.ReplaceAll(param, "False", "false")
			line = fmt.Sprintf("%s = toBool(%s)", lhs, param)
		}
		line = strings.ReplaceAll(line, "ñ", "nil")
		line = strings.ReplaceAll(line, "True", "true")
		line = strings.ReplaceAll(line, "False", "false")
		if strings.Contains(line, "=") {
			eqParts := strings.SplitN(line, "=", 2)
			if len(eqParts) == 2 {
				lhs := strings.TrimSpace(eqParts[0])
				rhs := strings.TrimSpace(eqParts[1])
				if strings.HasPrefix(rhs, "[") && strings.HasSuffix(rhs, "]") {
					rhs = "[]any{" + rhs[1:len(rhs)-1] + "}"
				}
				line = fmt.Sprintf("%s = %s", lhs, rhs)
			}
			*translatedLines = append(*translatedLines, fmt.Sprintf("\t%s", line))
		} else {
			*translatedLines = append(*translatedLines, fmt.Sprintf("\t%s", line))
		}
	}
}

func generateAndRunGoFile(imports []string, codeLines []string) {
	var sb strings.Builder
	sb.WriteString("package main\n\nimport (\n")
	
	added := make(map[string]bool)
	added["\"fmt\""] = true
	added["\"bufio\""] = true
	added["\"os\""] = true
	added["\"strings\""] = true
	added["\"strconv\""] = true
	added["\"math\""] = true
	added["\"math/rand\""] = true

	sb.WriteString("\t\"fmt\"\n")
	sb.WriteString("\t\"bufio\"\n")
	sb.WriteString("\t\"os\"\n")
	sb.WriteString("\t\"strings\"\n")
	sb.WriteString("\t\"strconv\"\n")
	sb.WriteString("\t\"math\"\n")
	sb.WriteString("\t\"math/rand\"\n")

	needJson := false
	for _, line := range codeLines {
		if strings.Contains(line, "json.") {
			needJson = true
			break
		}
	}

	if needJson {
		sb.WriteString("\t\"encoding/json\"\n")
		added["\"encoding/json\""] = true
	}

	for _, imp := range imports {
		if !added[imp] {
			sb.WriteString(fmt.Sprintf("\t%s\n", imp))
			added[imp] = true
		}
	}
	sb.WriteString(")\n\n")

	sb.WriteString(`func input(prompt string) string {
    fmt.Print(prompt)
    reader := bufio.NewReader(os.Stdin)
    text, _ := reader.ReadString('\n')
    return strings.TrimSpace(text)
}

func lower(s string) string {
    return strings.ToLower(s)
}

func upper(s string) string {
    return strings.ToUpper(s)
}

func ivs(s string) string {
    var sb strings.Builder
    for _, r := range s {
        if r >= 'a' && r <= 'z' {
            sb.WriteRune(r - 32)
        } else if r >= 'A' && r <= 'Z' {
            sb.WriteRune(r + 32)
        } else {
            sb.WriteRune(r)
        }
    }
    return sb.String()
}

func toString(v any) string {
    return fmt.Sprint(v)
}

func toInt(v any) int {
    switch val := v.(type) {
    case int:
        return val
    case int64:
        return int(val)
    case float64:
        return int(val)
    case string:
        i, err := strconv.Atoi(val)
        if err != nil {
            return 0
        }
        return i
    default:
        return 0
    }
}

func toFloat(v any) float64 {
    switch val := v.(type) {
    case float64:
        return val
    case int:
        return float64(val)
    case int64:
        return float64(val)
    case string:
        f, err := strconv.ParseFloat(val, 64)
        if err != nil {
            return 0.0
        }
        return f
    default:
        return 0.0
    }
}

func toBool(v any) bool {
    switch val := v.(type) {
    case bool:
        return val
    case string:
        if val == "" {
            return false
        }
        for _, r := range val {
            if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
                return false
            }
        }
        return true
    case int:
        return val > 0
    case int64:
        return val > 0
    case float64:
        return val > 0.0
    case float32:
        return float64(val) > 0.0
    default:
        return false
    }
}

func round(v any) int {
    switch val := v.(type) {
    case float64:
        return int(math.Round(val))
    case int:
        return val
    case int64:
        return int(val)
    case string:
        f, err := strconv.ParseFloat(val, 64)
        if err != nil {
            return 0
        }
        return int(math.Round(f))
    default:
        return 0
    }
}

func choice(v any) any {
    switch list := v.(type) {
    case []any:
        if len(list) == 0 {
            return nil
        }
        return list[rand.Intn(len(list))]
    case []string:
        if len(list) == 0 {
            return ""
        }
        return list[rand.Intn(len(list))]
    case []int:
        if len(list) == 0 {
            return 0
        }
        return list[rand.Intn(len(list))]
    default:
        return nil
    }
}

func randint(min int, max int) int {
    if min > max {
        min, max = max, min
    }
    return rand.Intn(max-min+1) + min
}

func randfloat(minVal any, maxVal any) float64 {
    min := toFloat(minVal)
    max := toFloat(maxVal)
    if min > max {
        min, max = max, min
    }
    return min + rand.Float64()*(max-min)
}

`)

	var functions []string
	var mainStatements []string
	isInsideFunc := false

	for _, line := range codeLines {
		if strings.HasPrefix(line, "type ") || strings.HasPrefix(line, "func ") {
			isInsideFunc = true
		}
		
		if isInsideFunc {
			functions = append(functions, line)
			if line == "}" {
				isInsideFunc = false
			}
		} else {
			mainStatements = append(mainStatements, line)
		}
	}

	for _, fnLine := range functions {
		sb.WriteString(fnLine + "\n")
	}

	sb.WriteString("func main() {\n")
	for _, stmt := range mainStatements {
		if stmt == "" {
			continue
		}
		if strings.HasPrefix(stmt, "\t") || stmt == "}" || strings.HasPrefix(stmt, "if") || strings.HasPrefix(stmt, "else") {
			sb.WriteString(fmt.Sprintf("%s\n", stmt))
		} else {
			sb.WriteString(fmt.Sprintf("\t%s\n", stmt))
		}
	}
	sb.WriteString("}\n")

	tempFileName := "temp_main.go"
	os.WriteFile(tempFileName, []byte(sb.String()), 0644)

	cmd := exec.Command("go", "run", tempFileName)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	
	_ = cmd.Run()
	os.Remove(tempFileName)
}
