import { Dispatch, SetStateAction, useEffect, useRef, useState } from 'react';
import { endpoint } from "../../App";
import { Tooltip as ReactTooltip } from 'react-tooltip';
import Editor, { loader } from '@monaco-editor/react';
import { languages } from 'monaco-editor';
import { GetSavedStrategiesResponse, GoogleAuthUser } from '../../models';
import formStyles from './Form.module.css'
import { FormViewProps } from './Form';
import { parseDateString } from '../../util';


export function FactorExpressionInput({
  user,
  userID,
  factorExpression,
  setFactorExpression,
  updateName,
  gptInput,
  selectedFactor,
  setSelectedFactor,
  setGptInput,
  savedStrategies,
  formProps,
}: {
  userID: string;
  factorExpression: string;
  setFactorExpression: Dispatch<SetStateAction<string>>;
  updateName: (arg: string) => void;
  user: GoogleAuthUser | null;
  gptInput: string;
  selectedFactor: string;
  setSelectedFactor: Dispatch<SetStateAction<string>>;
  setGptInput: Dispatch<SetStateAction<string>>;
  savedStrategies: GetSavedStrategiesResponse[];
  formProps: FormViewProps;
}) {
  const [err, setErr] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const gptInputElement = document.getElementById("gpt-input");

  const autofillEquation = async (e: any) => {
    e.preventDefault();
    setLoading(true);
    setFactorExpression("");
    try {
      const response = await fetch(endpoint + "/constructFactorEquation", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": user ? "Bearer " + user.accessToken : ""
        },
        body: JSON.stringify({ input: gptInput, userID })
      });
      setLoading(false);
      if (response.ok) {
        const result = await response.json();
        if (result.error.length === 0) {
          setFactorExpression(result.factorExpression);
          updateName(result.factorName);
        } else {
          setErr(result.error + " - " + result.reason);
        }
      } else {
        const j = await response.json();
        setErr(j.error + " - " + j.reason);
        console.error("Error submitting data:", response.status);
      }
    } catch (error) {
      setLoading(false);
      setErr((error as Error).message);
      console.error("Error:", error);
    }
  };

  if (err) {
    (gptInputElement as HTMLInputElement)?.setCustomValidity(err);
    (gptInputElement as HTMLInputElement).reportValidity();
  }

  useEffect(() => {
    setErr(null);
    (gptInputElement as HTMLInputElement)?.setCustomValidity("");
  }, [gptInput]);

  return <>
    <div>

      <label className={formStyles.label} style={{ position: "relative", width: "fit-content" }}>Factor Expression
        {/* <a
        data-tooltip-id="my-tfooltip"
        data-tooltip-content="The equation that will be run on every asset in the universe, on every rebalance date. Higher scoring assets will have a larger allocation in the portfolio."
        data-tooltip-place="bottom"
        style={{
          paddingLeft: "0px",
          marginTop: "2px",
          height: "100%",
          // fontSize
          position: "absolute",
          "right": "-18px",
          top: "-0.5px",
          fontSize: "14px"
        }}
      >
        <AiOutlineQuestionCircle style={{}} className="question-icon" />
      </a> */}
      </label>

      <ReactTooltip style={{ fontSize: "12px", maxWidth: "220px" }} id="my-tfooltip" />
      <p className={formStyles.label_subtext}>Select predefined factors or create your own.</p>

      <FactorPresetSelector
        setSelectedFactor={setSelectedFactor}
        setFactorExpression={setFactorExpression}
        updateName={updateName}
        selectedFactor={selectedFactor}
        savedStrategies={savedStrategies}
        formProps={formProps}
      />
      {selectedFactor === "gpt" ? <>
        <p style={{ marginTop: "5px" }} className={formStyles.label_subtext}>Uses ChatGPT API to convert factor description to equation.</p>
        <div className={formStyles.gpt_input_wrapper}>
          <textarea
            id="gpt-input"
            style={{
              width: "250px",
              height: "33px",
              fontSize: "13px"
            }}
            required={true}
            placeholder='small cap, undervalued, and price going up'
            value={gptInput}
            onChange={(e) => setGptInput(e.target.value)} />
          <button className={formStyles.gpt_submit} onClick={(e) => autofillEquation(e)}>➜</button>
        </div>
      </> : null}

      {selectedFactor === "gpt" ?
        <p style={{ marginTop: "5px", maxWidth: "380px" }} className={formStyles.label_subtext}>ChatGPT may determine incorrect equations. Be sure to double check and modify if necessary. <br /> <br />The equation applied to all assets, on each rebalance date. Higher scoring assets will have a larger allocation in the portfolio.</p>
        :
        <p className={formStyles.label_subtext} style={{ maxWidth: "380px", marginTop: "5px" }}>The equation applied to all assets, on each rebalance date. Higher scoring assets will have a larger allocation in the portfolio.</p>}

      <ExpressionEditor
        factorExpression={factorExpression}
        setFactorExpression={setFactorExpression}
        onChange={(newVal) => {
          setSelectedFactor("custom")
          formProps.updateName("custom")
        }}
      />



    </div>
  </>;
}

function FactorPresetSelector({
  setSelectedFactor,
  setFactorExpression,
  updateName,
  selectedFactor,
  savedStrategies,
  formProps,
}: {
  setSelectedFactor: Dispatch<SetStateAction<string>>,
  setFactorExpression: Dispatch<SetStateAction<string>>,
  updateName: (arg: string) => void,
  selectedFactor: string,
  savedStrategies: GetSavedStrategiesResponse[],
  formProps: FormViewProps,
}) {
  interface selectedFactorDetails {
    expression: string;
    factorName: string;
    update: () => void;
  }

  const presetMap: Record<string, selectedFactorDetails> = {
    "gpt": {
      expression: "",
      factorName: "",
      update: () => { },
    },
    "momentum": {
      expression: `pricePercentChange(
  nDaysAgo(7),
  currentDate
)`,
      factorName: "7_day_momentum",
      update: () => { },
    },
    "value": {
      expression: "10/pbRatio(currentDate)",
      factorName: "undervalued_by_pb_ratio",
      update: () => { },
    },
    "volatility": {
      expression: "1e3/stdev(nYearsAgo(1), currentDate)",
      factorName: "low_volatility",
      update: () => { },
    },
    "size": {
      expression: "1e12/marketCap(currentDate)",
      factorName: "small_cap",
      update: () => { },
    },
    "custom": {
      expression: `(
  (
    pricePercentChange(
      addDate(currentDate, 0, -6, 0),
      currentDate
    ) + pricePercentChange(
      addDate(currentDate, 0, -12, 0),
      currentDate
    ) + pricePercentChange(
      addDate(currentDate, 0, -18, 0),
      currentDate
    )
  ) / 3
) / stdev(addDate(currentDate, -3, 0, 0), currentDate)`,
      factorName: "custom",
      update: () => { },
    }
  };

  savedStrategies.forEach(s => presetMap[s.strategyName] = {
    factorName: s.strategyName,
    expression: s.factorExpression,
    update: () => {
      // formProps.setBacktestStart(parseDateString(s.backtestStart))
      // formProps.setBacktestEnd(parseDateString(s.backtestEnd))
      formProps.setAssetUniverse(s.assetUniverse)
      formProps.setNumSymbols(s.numAssets)
    },
  })

  const savedOptions = savedStrategies.length > 0 ? <optgroup label="Bookmarked Strategies">
    {savedStrategies.map((s, i) => <option key={i}>
      {s.strategyName}
    </option>)}
  </optgroup> : null;

  return <select
    onChange={(e) => {
      setSelectedFactor(e.target.value);
      setFactorExpression(presetMap[e.target.value].expression);
      presetMap[e.target.value].update()
      if (e.target.value !== "gpt") {
        updateName(presetMap[e.target.value].factorName);
      }
    }}
    value={selectedFactor}
    style={{ fontSize: "14px" }}
  >
    <optgroup label="Common Factors">
      <option value="momentum">Momentum (price trending up)</option>
      <option value="value">Value (undervalued relative to price)</option>
      <option value="size">Size (smaller assets by market cap)</option>
      <option value="volatility">Volatility (low risk assets)</option>
    </optgroup>
    {savedOptions}
    <option value="custom">Custom</option>
    <option value="gpt">Describe factor in words (ChatGPT)</option>
  </select>;
}

function ExpressionEditor({ factorExpression, setFactorExpression, onChange }: {
  factorExpression: string;
  onChange: (newVal: string) => void;
  setFactorExpression: Dispatch<SetStateAction<string>>;
}) {

  async function init() {
    const monaco = await loader.init();
    monaco.languages.register({ id: "mathLangxx" });
    monaco.languages.setMonarchTokensProvider("mathLangxx", mathLanguage);
    monaco.languages.setLanguageConfiguration("mathLangxx", conf)
    // monaco.languages.registerCompletionItemProvider('customLang', {
    //   provideCompletionItems: () => {
    //       return {
    //           suggestions: [
    //             'nDaysAgo', 'nMonthsAgo', 'nYearsAgo', 
    //             'price', 'pricePercentChange', 'stdev', 
    //             'pbRatio', 'peRatio', 'marketCap', 'eps', 
    //             'currentDate'
    //         ].map(keyword => ({ label: keyword, kind: monaco.languages.CompletionItemKind.Keyword }))
    //       };
    //   }
    // })
  }

  useEffect(() => {
    init()
  }, []);

  return (
    <div style={{ height: "150px", width: "300px", border: "0.5px solid rgba(63, 63, 63, 0.4)", resize: "both", overflow: "scroll", maxWidth: "100%" }}>
      <Editor
        height="100%"
        options={{
          lineNumbers: "off",
          minimap: {
            enabled: false
          },
          scrollbar: {
            vertical: 'hidden',
            horizontal: 'hidden'
          },
          scrollBeyondLastLine: false,
          hideCursorInOverviewRuler: true,
          overviewRulerLanes: 0,
          contextmenu: false,
          // renderLineHighlight: "none",
          // remove left margin
          glyphMargin: false,
          folding: false,
          lineDecorationsWidth: 8,
        }}
        width="100%"
        language="mathLangxx"
        value={factorExpression}
        onChange={(e) => {
          // e ? setFactorExpression(e) : ""
          if (e) {
            setFactorExpression(e)
            onChange(e)
          }
        }}
      />
    </div>

  )
}

// monaco language config

export const conf: languages.LanguageConfiguration = {
  comments: {
    lineComment: '//',
    blockComment: ['/*', '*/']
  },
  brackets: [
    ['{', '}'],
    ['[', ']'],
    ['(', ')']
  ],
  autoClosingPairs: [
    { open: '{', close: '}' },
    { open: '[', close: ']' },
    { open: '(', close: ')' },
    { open: '"', close: '"' }
  ],
  surroundingPairs: [
    { open: '{', close: '}' },
    { open: '[', close: ']' },
    { open: '(', close: ')' },
    { open: '"', close: '"' }
  ]
};

export const mathLanguage: languages.IMonarchLanguage = {
  defaultToken: 'invalid',
  tokenPostfix: '.customLang',

  // Regular expressions
  keywords: [

    'currentDate'
  ],

  functions: ['nDaysAgo', 'nMonthsAgo', 'nYearsAgo',
    'price', 'pricePercentChange', 'stdev',
    'pbRatio', 'peRatio', 'marketCap', 'eps', 'addDate'],

  typeKeywords: [
    'strDate'
  ],

  operators: [
    '+', '-', '*', '/', '(', ')'
  ],

  symbols: /[=><!~?:&|+\-*\/\^%]+/,

  // The main tokenizer for our languages
  tokenizer: {
    root: [
      // identifiers and keywords
      [/[a-zA-Z_]\w*/, {
        cases: {
          '@functions': 'function',
          '@keywords': 'keyword',
          '@typeKeywords': 'type',
          // '@default': 'invalid',
        }
      }],
      // [/[A-Z][\w\$]*/, 'type.identifier'],  // to show class names nicely

      // functions
      // [/(nDaysAgo|nMonthsAgo|nYearsAgo|price|pricePercentChange|stdev|pbRatio|peRatio|marketCap|eps)\b/, 'type.identifier'],

      // numbers
      // [/\d*\.\d+([eE][\-+]?\d+)?/, 'number.float'],
      [/\d*\d+[eE]([\-+]?\d+)?/, 'number.float'],
      [/\d*\.\d+([eE][\-+]?\d+)?/, 'number.float'],
      [/\d+/, 'number'],
      [/-\d+/, 'number'],


      [/[;,.]/, 'delimiter'],

      // whitespace
      { include: '@whitespace' },

      // delimiters and operators
      // [/[{}()\[\]]/, '@brackets'],
      [/[<>](?!@symbols)/, '@operators'],
      [/@symbols/, 'delimiter'],

      // strings
      [/"([^"\\]|\\.)*$/, 'string.invalid'],  // non-terminated string
      [/"/, 'string', '@string'],

      // characters
      [/'[^\\']'/, 'string'],
      [/'/, 'string.invalid']
    ],

    whitespace: [
      [/[ \t\r\n]+/, ''],
      [/\/\*\*(?!\/)/, 'comment.doc', '@doc'],
      [/\/\*/, 'comment', '@comment'],
      [/\/\/.*$/, 'comment']
    ],

    comment: [
      [/[^/*]+/, 'comment'],
      [/\/\*/, 'comment', '@push'],
      [/\*\//, 'comment', '@pop'],
      [/[\/*]/, 'comment']
    ],

    string: [
      [/[^\\"]+/, 'string'],
      [/\\./, 'string.escape.invalid'],
      [/"/, 'string', '@pop']
    ],

    doc: [
      [/[^*/]+/, 'comment.doc'],
      [/\*\//, 'comment.doc', '@pop'],
      [/./, 'comment.doc']
    ]
  }
}